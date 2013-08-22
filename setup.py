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
      author='Sergey Obukhov',
      author_email='sergey.obukhov@rackspace.com',
      url='https://github.com/mailgun/vulcan',
      license='APACHE2',
      packages=find_packages(exclude=['tests']),
      include_package_data=True,
      zip_safe=True,
      install_requires=[
          'setproctitle==1.1.7',
          'Twisted==13.1.0',
          # required by telephus
          'pure-sasl==0.1.4',
          'expiringdict==1.0',
          'regex==0.1.20110315',
          'thrift==0.9.1',
          'telephus==1.0.0-beta1',
          'nose==1.3.0',
          'mock==1.0.1',
          'coverage==3.6'
          ],
      dependency_links=[
          ('https://github.com/mailgun/expiringdict/tarball/'
           'master#egg=expiringdict-1.0'),
          ('https://github.com/driftx/Telephus/tarball/'
           'master#egg=telephus-1.0.0-beta1')
          ]

      )


if __name__ == '__main__':
    if any(cmd in sys.argv[1:] for cmd in ['develop', 'install', 'bdist_egg']):
        # adding github repo to dependency_links and requiring treq version
        # higher than on PyPi won't work for packages that require vulcan
        # so we explicitly install it from github here
        pip("install https://github.com/klizhentas/treq/tarball/"
            "b7e40c23108c810d81641d38a12e900bf4f2e599/master#egg=treq-0.2.0")
