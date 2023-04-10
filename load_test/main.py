import logging
import os
import random
import string
import threading
import time
from typing import List

from file_server_client import FileServerClient
from rate_limiter import RateLimiter
from results import RequestResult, ResultStats

root = logging.getLogger()
root.setLevel(logging.INFO)  # Set to logging.DEBUG for much more infomation

# Configure the load test here
FILE_SERVER_ADDR = os.getenv("FILE_SERVER_ADDR", default="http://localhost:1234")
FILE_SERVER_PREFIX = "api/fileserver"
MAX_NUMBER_OF_FILES = 500
MAX_FILE_SIZE_BYTES = 1024
REQUESTS_PER_SECOND: int = 100  # Max requests per second that the load test will run.

# Globals
CLIENT_ID = "fileserver_load_tester"
NUMBER_OF_FILES = 0
TRACKED_FILES: List[str] = []

# Singeltons
RESULT_STATS: ResultStats = ResultStats()
KEEP_RUNNING = True
RATE_LIMITER = RateLimiter(throughput_per_second=REQUESTS_PER_SECOND, burst_balance_maximum=0,
                           burst_balance_reload_interval=0)
FILE_SERVER_CLIENT = FileServerClient(FILE_SERVER_ADDR, FILE_SERVER_PREFIX, MAX_FILE_SIZE_BYTES)

def perform_random_fileserver_action() -> RequestResult:
    # As NUMBER_OF_FILES approaches MAX_NUMBER_OF_FILES, reduce likelihood of creating new files
    create_new_file = random.randint(0, MAX_NUMBER_OF_FILES) > NUMBER_OF_FILES
    file_name = random.choice(TRACKED_FILES) if len(TRACKED_FILES) > 0 else ""

    # Selecdt a random file operation to run
    funcs = [FILE_SERVER_CLIENT.put_file, FILE_SERVER_CLIENT.get_file, FILE_SERVER_CLIENT.delete_file]
    to_execute = random.choice(funcs)

    if create_new_file or not file_name:
        file_name = ''.join(random.choices(string.ascii_letters, k=12))
        to_execute = FILE_SERVER_CLIENT.put_file

    return to_execute(file_name=file_name)


def run_load_test():
    global KEEP_RUNNING, RATE_LIMITER
    try:
        while KEEP_RUNNING:
            if RATE_LIMITER.is_allowed(CLIENT_ID):
                result: RequestResult = perform_random_fileserver_action()
                RESULT_STATS.merge(result)

            # logging.info(rate_limiter.get_clients())
            time.sleep(.1)
            RATE_LIMITER.log_stats()
    except Exception as e:
        KEEP_RUNNING = False
        logging.error(f"IRRECOVERABLE ERROR: Got unexpected exception: {e}. EXITING.")
        logging.fatal(e)


NUM_WORKERS = int(REQUESTS_PER_SECOND / 3) + 1
threads = []

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


