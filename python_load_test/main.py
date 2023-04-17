import logging
import os
import random
import string
import threading
import time
from typing import List, Set

from file_server_test_client import FileServerTestClient
from rate_limiter import RateLimiter
from results import TestResult, ResultStats

root = logging.getLogger()
root.setLevel(os.getenv("LOG_LEVEL", default=logging.INFO))  # Set to logging.DEBUG for much more information
fileHandler = logging.FileHandler("/tmp/load_test.log")
fileHandler.setLevel(logging.INFO)
root.addHandler(fileHandler)

# Configure the load test here
FILE_SERVER_ADDR: str = os.getenv("FILE_SERVER_ADDR", default="http://localhost:1234")
FILE_SERVER_PREFIX: str = "api/fileserver"
MAX_NUMBER_OF_FILES: int = int(os.getenv("MAX_FILE_COUNT", default=50))
MAX_FILE_SIZE_BYTES: int = int(os.getenv("MAX_FILE_SIZE", default=1024))
REQUESTS_PER_SECOND: int = int(
    os.getenv("REQUESTS_PER_SECOND", default=25))  # Max requests per second that the load test will run.

# Globals
CLIENT_ID = "fileserver_load_tester"

# Global Singeltons
RESULT_STATS: ResultStats = ResultStats(REQUESTS_PER_SECOND, MAX_NUMBER_OF_FILES, MAX_FILE_SIZE_BYTES)
RATE_LIMITER = RateLimiter(throughput_per_second=REQUESTS_PER_SECOND, burst_balance_maximum=0,
                           burst_balance_reload_interval=0)
FILE_SERVER_CLIENT = FileServerTestClient(FILE_SERVER_ADDR, FILE_SERVER_PREFIX, MAX_FILE_SIZE_BYTES)

KEEP_RUNNING = True

# Make load test default to be read heavy which more closely simulates real world traffic
POSSIBLE_TESTS = [FILE_SERVER_CLIENT.delete_file, FILE_SERVER_CLIENT.put_file, FILE_SERVER_CLIENT.put_file]
for i in range(0, 75):
    POSSIBLE_TESTS.append(FILE_SERVER_CLIENT.get_file)

def perform_random_fileserver_action() -> TestResult:
    global POSSIBLE_TESTS
    # As NUMBER_OF_FILES approaches MAX_NUMBER_OF_FILES, reduce likelihood of creating new files
    create_new_file = random.randint(0, MAX_NUMBER_OF_FILES) > FILE_SERVER_CLIENT.tracked_count()
    file_name = ""
    to_execute = FILE_SERVER_CLIENT.put_file

    if not create_new_file:
        file_name = FILE_SERVER_CLIENT.get_random_not_in_process_file()
        # Select a random file operation to run, make get more common than delete, so we add it multiple times here
        to_execute = random.choice(POSSIBLE_TESTS)

    is_create = False
    if create_new_file or not file_name:
        if create_new_file and FILE_SERVER_CLIENT.tracked_count() < MAX_NUMBER_OF_FILES:
            is_create = True
            logging.debug(f"Got number of files: {FILE_SERVER_CLIENT.tracked_count()}, creating new file.")
            file_name = ''.join(random.choices(string.ascii_letters, k=12))
        to_execute = FILE_SERVER_CLIENT.put_file

    try:
        return to_execute(file_name=file_name)
    except Exception as e:
        logging.exception(e)
        RESULT_STATS.other_errors.append(f"{e}")


def run_load_test():
    global KEEP_RUNNING, RATE_LIMITER
    try:
        while KEEP_RUNNING:
            if RATE_LIMITER.is_allowed(CLIENT_ID):
                result: TestResult = perform_random_fileserver_action()
                RESULT_STATS.merge(result)

            # logging.info(rate_limiter.get_clients())
            time.sleep(.01)
    except Exception as e:
        KEEP_RUNNING = False
        RESULT_STATS.other_errors.append(f"{e}")
        logging.error(f"IRRECOVERABLE ERROR: Got unexpected exception: {e}. EXITING.")
        logging.fatal(e)


NUM_WORKERS = int(REQUESTS_PER_SECOND / 3)
threads = []

logging.info(f"Starting load test attempting {REQUESTS_PER_SECOND} target throughput.")
logging.info(f"Spawning {NUM_WORKERS} worker threads.")

for i in range(0, NUM_WORKERS):
    logging.debug(f"Starting thread: {i}")
    thread = threading.Thread(target=run_load_test, args=())
    thread.start()
    threads.append(thread)

try:
    while KEEP_RUNNING:
        time.sleep(1)
        os.system('cls' if os.name == 'nt' else 'clear')
        RESULT_STATS.print_stats()

except KeyboardInterrupt:
    KEEP_RUNNING = False
    logging.info("\n\n Error dump: \n")

    for err in RESULT_STATS.http_errors:
        logging.info(f"HTTP error from FS: {err}")

    for err in RESULT_STATS.other_errors:
        logging.info(f"Other err:{err}")
