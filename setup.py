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
        'twisted==12.2.0',
        # to convert strings to file-like objects
        # 'Werkzeug==0.8.3',
        # required by telephus
        # 'pure-sasl==0.1.4',
        'expiringdict==1.0',
        'regex==0.1.20110315',
        ],
      extras_require={'test': ['nose', 'mock', 'coverage']},
      dependency_links=[
          ('https://{u}:{p}@github.com/mailgun/expiringdict/tarball/'
           'master#egg=expiringdict-1.0').format(
              u=quote(env.get('MG_COLABORATOR')),
              p=quote(env.get('MG_COLABORATOR_PASSWORD'))),
          ]

      )


# TODO install thrift==1.0.0-dev
# TODO install telephus==1.0.0-beta1
# TODO install Cassandra 1.2.5 (using chef)


if __name__ == '__main__':
    if len(sys.argv) > 1 and sys.argv[1] in ['develop', 'install']:
        # adding github repo to dependency_links and requiring treq version
        # higher than on PyPi won't work for packages that require vulcan
        # so w explicitly install it from github here
        pip("install https://github.com/klizhentas/treq/tarball/"
            "b7e40c23108c810d81641d38a12e900bf4f2e599/master#egg=treq-0.2.0")
