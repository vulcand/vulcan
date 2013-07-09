"""
Simple internal ESMTP implementation taken from Twisted examples:
    http://twistedmatrix.com/documents/current/mail/examples/emailserver.tac
"""
import json
# importing from cStringIO won't work for treq
from StringIO import StringIO
from time import time
import socket
from email.utils import formatdate
from functools import partial

from zope.interface import implements
from werkzeug.datastructures import FileStorage
import treq

from twisted.internet import defer, threads
from twisted.mail import smtp
from twisted.mail.imap4 import LOGINCredentials, PLAINCredentials
from twisted.cred.credentials import UsernamePassword
from twisted.names.client import lookupPointer
from twisted.python import failure
from twisted.cred import credentials
from twisted.cred.checkers import ICredentialsChecker
from twisted.cred.portal import IRealm, Portal
from twisted.internet.defer import maybeDeferred

from vulcan.utils import is_valid_ip
from vulcan.constraints import MAX_MESSAGE_SIZE_BYTES
from vulcan.auth import authorize
from vulcan import log
from vulcan import config


SMTP_TRANSACTION_FAILED = 554
UNHANDLED_SMTP_ERROR = "Mailgun cannot accept your message"
SMTP_RELAY_DENIED = "5.7.1 Relaying denied"


def my_hostname():
    '''Returns a short hostname for this machine, for example
    for turret4.mailgun.net will return "turret4"'''
    return socket.gethostname().split('.')[0]


def make_esmtp_id(helo):
    '''Generates a unique string for this ESMTP session'''
    return '%x.%x-%s' % (time(), id(helo), my_hostname())


class CredentialsChecker(object):
    '''
    This class is needed to enable AUTH command handling via Twisted.

    Normally nginx will not relay AUTH command to the back-end (us), instead
    it will do his own authentication.

    But if a user issues another (!) SMTP AUTH, then nginx starts relaying.
    This means that authentication needs to be performed in two places:

        1. api/hidden/smtpin.py:nginx_smtp_auth()
        2. here, inside of requestAvatarId()
    '''
    implements(ICredentialsChecker)
    credentialInterfaces = (credentials.IUsernamePassword,
                            credentials.IUsernameHashedPassword)

    def requestAvatarId(self, creds):
        '''This is where checking for username + password happens'''
        d = maybeDeferred(authorize,
                          dict(username=creds.username,
                               password=creds.password,
                               protocol="SMTP",
                               length=None))
        d.addCallback(partial(self.authorizationReceived, d, creds))
        d.addErrback(partial(self.communicationFailed, d))
        return d

    def communicationFailed(self, d, reason):
        log.exception(reason.getTraceback())
        return d.errback(
            smtp.SMTPServerError(
                SMTP_TRANSACTION_FAILED, UNHANDLED_SMTP_ERROR))

    def authenticationFailed(self, d, reason=None):
        log.exception(reason.getTraceback())
        return d.errback(
            smtp.AUTHDeclinedError(535, '5.7.0 authentication failed'))

    def authorizationReceived(self, d, creds, response):
        # authorization succeeded
        # authorization object fetched from cache
        if isinstance(response, dict):
            return d.callback([creds, response])
        # authorization failed
        # codes below 400 are considered to be OK
        # e.g. 200, 201, 301, 302 are all OK
        elif response.code >= 400:
            # TODO fetch the reason from server
            return self.authenticationFailed(d)
        else:
            content = treq.json_content(response)
            content.addcallback(partial(self.avatarIdReady, d, creds))
            content.addErrback(partial(self.communicationFailed, d))
            return content

    def avatarIdReady(self, d, creds, settings):
        return d.callback([creds, settings])


class SmtpMessage(object):
    implements(smtp.IMessage)

    def __init__(self, delivery=None):
        self.lines = []
        self.delivery = delivery

    def toHTTPRequest(self, mime_message):
        return treq.post(
            ("http://" +
             pick_server(get_servers(self.delivery.settings["upstream"])) +
             self.delivery.settings['http']['path']),
            data={"to": ",".join(recipients)},
            files={"message": FileStorage(stream=StringIO(mime_message),
                                          name="mime")})

    def lineReceived(self, line):
        if self.delivery:
            self.lines.append(line)

    def eomReceived(self):
        '''
        The entire message has been finally received.
        '''
        # message without delivery? can't deliver:
        if not self.delivery:
            return defer.succeed(None)

        creds = self.delivery.creds
        if creds:
            auth_ip = creds.password or ''  # yes, we store IP as 'password'
        else:
            auth_ip = ''

        return self.pointerLookup(auth_ip)

    def pointerLookup(self, ip):
        addr = '{}.in-addr.arpa'.format('.'.join(reversed(ip.split('.'))))
        later = defer.Deferred()
        lookupPointer(addr).addBoth(
            partial(self.pointerLookupCompleted, later))
        return later

    def pointerLookupCompleted(self, later, result):
        rdns = 'Unknown'
        try:
            if not isinstance(result, failure.Failure):
                if result[0]:
                    rdns = result[0][0].payload.name
        except Exception, e:
            log.exception(e)
        self.submit(later, rdns)

    def submit(self, later, rdns):
        auth_user = None
        auth_ip = None

        try:
            helo = self.delivery.helo
            creds = self.delivery.creds
            if creds:
                auth_user = creds.username
                auth_ip = creds.password  # yes, we store IP as 'password'

            # Add "Received" header:
            if auth_ip and is_valid_ip(auth_ip) and helo:
                rcv = (
                    "Received: from {} ({} [{}])\n "
                    "by mxa.mailgun.org with ESMTP id {};\n {} (UTC)").format(
                    helo,
                    rdns,
                    auth_ip,
                    make_esmtp_id(helo),
                    formatdate(usegmt=False))
                self.lines.insert(0, rcv)

            # Add "X-Envelope-From" header for non-authenticated users
            origin = str(self.delivery.origin)
            if not auth_user:
                self.lines.insert(0, 'X-Envelope-From: <%s>' % origin)

            # self.delivery.recipients is actualy an array of
            # twisted.mail.smtp.User objects that also have various useful
            # information there, like EHLO data
            recipients = list(set([str(r) for r in self.delivery.recipients]))
            mime_message = "\r\n".join(self.lines)

            # erase this message and reset the session
            # (in case there will be another message after us):
            self.lines = []
            if self.delivery:
                self.delivery.message = None
                self.delivery.recipients = []
                self.delivery.data_length = 0

            log.info("%s accept_via_smtp(rcpt=%s, msglen=%d)" % (
                auth_ip, recipients, len(mime_message)))

            d = self.toHTTPRequest(mime_message)

            d.addCallbacks(partial(self.httpResponseReceived, later))
            d.addErrback(partial(self.communicationFailed, later))
            return d

        # Unhandled error:
        except Exception:
            log.exception("Error in Message.eomReceived()")
            return later.errback(
                smtp.SMTPServerError(
                    SMTP_TRANSACTION_FAILED, UNHANDLED_SMTP_ERROR))

    def communicationFailed(self, later, reason):
        log.exception(reason.getTraceback())
        return later.errback(
            smtp.SMTPServerError(
                SMTP_TRANSACTION_FAILED, json.loads(result)["message"]))

    def httpResponseReceived(self, later, response):
        d = treq.content(result)
        d.addCallback(partial(self.submitSucceeded, later))
        d.addErrback(partial(self.communicationFailed, later))
        return d

    def submitSucceeded(self, later, response):
        if response.code < 400:
            return later.callback(None)
        else:
            try:
                result = json.loads(result)
            except:
                result = {}

            if "message" in result:
                return later.errback(
                    smtp.SMTPServerError(
                        SMTP_TRANSACTION_FAILED,
                        result.get("message", UNHANDLED_SMTP_ERROR)))

    def connectionLost(self):
        # There was an error, throw away the stored lines
        self.lines = []


class MessageDelivery(object):
    implements(smtp.IMessageDelivery)

    def __init__(self, creds=None, settings=None):
        self.settings = settings
        self.creds = creds
        self.recipients = []
        self.message = None
        # SMTP DATA length
        self.data_length = 0

    def receivedHeader(self, helo, origin, recipients):
        self.recipients += recipients

    def validateFrom(self, helo, origin):
        # All addresses are accepted
        self.origin = origin
        if helo:
            self.helo = helo[0]
        else:
            self.helo = 'unknown'
        return origin

    def validateTo(self, rcpt_to):
        '''
        Gets called for every incoming RCPT TO command. Validates the recipient
        and spits back SMTP errors.
        '''
        reject_reason = []
        auth_user = None
        if self.creds and self.creds.username:
            auth_user = self.creds.username

        d = treq.get(
            ("http://" +
             pick_server(get_servers('auxvalidation')) +
             config['validate_rcpt']),
            params={"auth_user": auth_user, "rcpt_to": str(rcpt_to)})

        d.addCallback(partial(self.validateToReceived, d, rcpt_to))
        d.addErrback(partial(self.communicationFailed, d))
        return d

    def communicationFailed(self, d, reason):
        log.exception(reason.getTraceback())
        return d.errback(smtp.SMTPServerError(SMTP_TRANSACTION_FAILED,
                                              UNHANDLED_SMTP_ERROR))

    def validateToReceived(self, d, rcpt_to, response):
        if response.code >= 400:
            return self.communicationFailed(d, response.code)
        else:
            content = treq.json_content(response)
            content.addCallback(partial(self.validatedTo, d, rcpt_to))
            return content

    def validatedTo(self, d, rcpt_to, result):
        if result.get("valid"):
            msg = defer.Deferred()
            msg.addCallback(partial(self.checkAndUpdateRate, msg))
            msg.addErrback(partial(self.communicationFailed, msg))
            return msg
        # recipient is not accepted:
        else:
            # TODO could reason end up being unicode?
            reason = result.get("message") or SMTP_RELAY_DENIED
            log.info(
                "SMTP RCPT TO: {} rejected: {}".format(
                    rcpt_to, reason))
            raise smtp.SMTPBadRcpt(rcpt_to, code=550, resp=str(reason))

    def checkAndUpdateRate(self, d, settings):
        request_params = dict(
            token=settings["token"],
            protocol='SMTP',
            limit=settings["limit"])

        rate_check = check_rate(request_params)
        rate_check.addCallback(partial(self.rateCheckReceived,
                                       d, settings["upstream"]))
        rate_check.addErrback(partial(self.communicationFailed, d))

        update_rate(request_params)

        return rate_check

    def rateCheckReceived(self, d, upstream):
        content = treq.json_content(response)
        content.addCallback(partial(self.handleRateCheckResult, d, upstream))
        content.addErrback(partial(self.communicationFailed, d))
        return content

    def handleRateCheckResult(self, d, upstream, result):
        if result.get("reached"):
            # TODO provide miningful error message here
            return d.errback(
                smtp.SMTPServerError(SMTP_TRANSACTION_FAILED,
                                     UNHANDLED_SMTP_ERROR))
        else:
            self.rate_check = result
            if not self.message:
                self.message = SmtpMessage(self)
                return lambda: self.message

            else:
                return lambda: SmtpMessage(None)


class SimpleRealm:
    '''Situation is calling for a simple realm. Here it comes,
    carrying an avatar!'''
    implements(IRealm)

    def requestAvatar(self, avatarId, mind, *interfaces):
        if smtp.IMessageDelivery in interfaces:
            return (smtp.IMessageDelivery,
                    MessageDelivery(*avatarId),
                    lambda: None)
        raise NotImplementedError()


class SmtpProtocol(smtp.ESMTP):
    # increase the max. size of SMTP line
    MAX_LENGTH = 16384 * 100

    @staticmethod
    def splitLines(data):
        return [line.rstrip("\r") for line in data.split("\n")]

    def dataReceived(self, data):
        """
        Translates bytes into lines, and calls lineReceived.
        NOTE: we copied this function from smtp.SMTP class because
              we needed to add support to non-standard newlines that
              some clients use (Redmine #3083)
        """
        if self.mode is smtp.DATA:
            # count the DATA length during smtp session
            self.delivery.data_length += len(data)
            if self.delivery.data_length >= MAX_MESSAGE_SIZE_BYTES:
                return self.messageLengthExceeded()

        # the next line is different from Twisted implementation. It
        # understands both CRLF and simply LF
        lines = self.splitLines(self._buffer+data)
        self._buffer = lines.pop(-1)
        for line in lines:
            if self.transport.disconnecting:
                # this is necessary because the transport may be told to lose
                # the connection by a line within a larger packet, and it is
                # important to disregard all the lines in that packet following
                # the one that told it to close.
                return
            if len(line) > self.MAX_LENGTH:
                return self.lineLengthExceeded(line)
            else:
                print 111111, line
                self.lineReceived(line)
        if len(self._buffer) > self.MAX_LENGTH:
            return self.lineLengthExceeded(self._buffer)

    def do_XCLIENT(self, line):
        '''
        This is a non-standard SMTP extension. XCLIENT command looks like this:
        "XCLIENT ADDR=211.10.3.1 LOGIN=postmaster@mailgun.org NAME=[domain.us]"
        '''
        self.xclient = dict()
        # parse XCLIENT line and store it as dict:
        for pair in line.split(' '):
            parts = pair.split('=')
            if len(parts) == 2:
                self.xclient[parts[0].upper()] = parts[1]

        self.delivery.creds = UsernamePassword(self.xclient.get('LOGIN', None),
                                               self.xclient.get('ADDR', None))
        self.sendCode(250, "OK")

    def do_QUIT(self, rest):
        '''Our own version of Twisted's QUIT. Differs only by our bye message.
        '''
        self.sendCode(221, 'See you later. Yours truly, Mailgun')
        self.transport.loseConnection()

    def messageLengthExceeded(self):
        '''Message is big enough to reject it.
        '''
        self.sendCode(
            SMTP_TRANSACTION_FAILED,
            "Message is too big. Max message size is {} bytes".format(
                MAX_MESSAGE_SIZE_BYTES))

    def _messageHandled(self, resultList):
        failures = []
        for (success, result) in resultList:
            if not success:
                failures.append(result)

        # Great success!
        if not failures:
            self.sendCode(250, 'Great success')
        else:
            try:
                failure = failures.pop()
                exception = failure.value

                # for SMTPServerError exceptions we return code/response
                # to the client:
                if isinstance(exception, smtp.SMTPServerError):
                    self.sendCode(exception.code, str(exception.resp))
                else:
                    # for others we return generic response:
                    msg = "Mailgun cannot accept your mail"
                    L = len(resultList)
                    if L > 1:
                        msg += (' (%d failures out of %d recipients)'
                                % (len(failures), L))
                    self.sendCode(554, msg)

            except Exception as e:
                self.sendCode(554, "Server error. Try again later")


class SMTPFactory(smtp.SMTPFactory):
    '''The purpose of a Twisted factory is to "build a protocol" object
    and return it from buildProtocol()'''
    protocol = SmtpProtocol

    def buildProtocol(self, addr):
        p = smtp.SMTPFactory.buildProtocol(self, addr)
        p.delivery = MessageDelivery()
        p.challengers = {"LOGIN": LOGINCredentials, "PLAIN": PLAINCredentials}
        return p
