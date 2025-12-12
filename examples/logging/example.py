#!/usr/bin/env scriptling

"""Example demonstrating the logging library functionality."""

import logging

print("=== Logging Library Example ===\n")

# 1. Basic logging using module-level functions
print("1. Basic logging with module functions:")
logging.debug('This is a debug message (may not show depending on level)')
logging.info('This is an info message')
logging.warning('This is a warning message')
logging.error('This is an error message')
logging.critical('This is a critical message')

print("\n" + "-"*50 + "\n")

# 2. Using a named logger
print("2. Using a named logger:")
logger = logging.getLogger('simpleExample')

logger.debug('debug message from simpleExample')
logger.info('info message from simpleExample')
logger.warning('warn message from simpleExample')
logger.error('error message from simpleExample')
logger.critical('critical message from simpleExample')

print("\n" + "-"*50 + "\n")

# 3. Multiple loggers
print("3. Multiple loggers for different components:")
app_logger = logging.getLogger('application')
db_logger = logging.getLogger('database')
api_logger = logging.getLogger('api')

app_logger.info('Application starting up')
db_logger.debug('Connecting to database')
api_logger.warning('Rate limit approaching')
db_logger.info('Database connection successful')
app_logger.error('Failed to load configuration')

print("\n" + "-"*50 + "\n")

# 4. Demonstrating warn() alias (Python compatibility)
print("4. Python compatibility - using warn() alias:")
logger = logging.getLogger('compat_test')
logger.warning('This is using warning()')
logger.warn('This is using warn() - same as warning()')

print("\n" + "-"*50 + "\n")

# 5. Accessing logging constants
print("5. Logging level constants:")
print(f"DEBUG = {logging.DEBUG}")
print(f"INFO = {logging.INFO}")
print(f"WARNING = {logging.WARNING}")
print(f"ERROR = {logging.ERROR}")
print(f"CRITICAL = {logging.CRITICAL}")

print("\n=== Example completed ===")