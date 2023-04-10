import dataclasses
import time
import logging
from typing import List


class RequestResult:
    def __init__(self, response):
        self.response = response

    def was_success(self) -> bool:
        return 200 <= self.response.status_code < 300

    def was_throttled(self) -> bool:
        return self.response.status_code == 429

    def was_error(self) -> bool:
        return self.response.status_code >= 400

    def error_message(self) -> str:
        return self.response.text


class ResultStats:

    def __init__(self, target_throughput: int, max_files: int, max_file_size: int):
        self.target_throughput = target_throughput
        self.max_files = max_files
        self.max_file_size = max_file_size
        self.start_time: float = time.time()
        self.total_requests: int = 0
        self.num_success: int = 0
        self.num_failure: int = 0
        self.num_throttled: int = 0
        self.http_errors: List[str] = []
        self.other_errors: List[str] = []

    def print_stats(self):
        throughput = round(self.total_requests / (time.time() - self.start_time), 1)
        success_throughput = round(self.num_success / (time.time() - self.start_time), 1)
        logging.info("Test Configuration:")
        logging.info("------------------------------------------------")
        logging.info(f"Target throughput: {self.target_throughput} req/sec")
        logging.info(f"Max # files: {self.max_files}")
        logging.info(f"Max file size: {self.max_file_size} bytes.")
        logging.info(f"")
        logging.info("Test Results:")
        logging.info("------------------------------------------------")
        logging.info(f"Attempted Throughput: {throughput} requests/sec")
        logging.info(f"Successful Throughput: {success_throughput} requests/sec")
        logging.info(f"Total requests: {self.total_requests}")
        logging.info(f"Total success: {self.num_success}")
        logging.info(f"Total failure: {self.num_failure}")
        logging.info(f"Total throttled: {self.num_throttled} by file server.")
        logging.info("")
        for err in self.http_errors[-5:]:
            logging.info(f"Error received from fileserver: {err}")

        logging.info("")
        for err in self.other_errors[-5:]:
            logging.info(err)

        # Prevent memory leak from error growth, only keep last 100 errors.
        if len(self.http_errors) > 100:
            self.http_errors = self.http_errors[-100:]

        if len(self.other_errors) > 100:
            self.http_errors = self.http_errors[-100:]

    def merge(self, result: RequestResult):
        self.total_requests = self.total_requests + 1

        if result.was_success():
            self.num_success = self.num_success + 1

        if result.was_error() and not result.was_throttled():
            self.num_failure = self.num_failure + 1
            self.http_errors.append(result.error_message())

        if result.was_throttled():
            self.num_throttled = self.num_throttled + 1
            self.http_errors.append(result.error_message())
