import sys
import os
from os import environ as env
from os.path import join, dirname
from urllib import quote
from setuptools import setup, find_packages


def run(command):
    if os.system(command) != 0:
        raise Exception("Failed '{}'".format(command))
    else:
        return 0


def pip(command):
    command = '{} {}'.format(join(dirname(sys.executable), 'pip'), command)
    run(command)


setup(name='vulcan',
      version='1.0',
      description="reverse proxy service",
      long_description=open("README.md").read(),
      author='Mailgun Inc.',
      author_email='admin@mailgunhq.com',
      url='http://www.mailgun.com',
      license='MIT',
      packages=find_packages(exclude=['tests']),
      include_package_data=True,
      zip_safe=True,
      install_requires=[
        'setproctitle',
        'twisted',
        # to convert strings to file-like objects
        # 'Werkzeug==0.8.3',
        # required by telephus
        'pure-sasl==0.1.4',
        'expiringdict==1.0',
        'regex==0.1.20110315',
        'thrift',
        'telephus==1.0.0-beta1'
        ],
      extras_require={'test': ['nose', 'mock', 'coverage']},
      dependency_links=[
          ('https://{u}:{p}@github.com/mailgun/expiringdict/tarball/'
           'master#egg=expiringdict-1.0').format(
              u=quote(env.get('MG_COLABORATOR')),
              p=quote(env.get('MG_COLABORATOR_PASSWORD'))),
          ('https://github.com/driftx/Telephus/tarball/'
           'master#egg=telephus-1.0.0-beta1')
          ]

      )


if __name__ == '__main__':
    if any(cmd in sys.argv[1:] for cmd in ['develop', 'install', 'bdist_egg']):
        # adding github repo to dependency_links and requiring treq version
        # higher than on PyPi won't work for packages that require vulcan
        # so w explicitly install it from github here
        pip("install https://github.com/klizhentas/treq/tarball/"
            "b7e40c23108c810d81641d38a12e900bf4f2e599/master#egg=treq-0.2.0")
