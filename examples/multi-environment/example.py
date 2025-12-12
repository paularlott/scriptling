import logging

print("Testing logging in this environment...")

# Create a logger and log a message
logger = logging.getLogger('test_env')
logger.info('This message should use the environment-specific logger')

# Test module-level logging
logging.warning('Warning from module level')

print("Done!")