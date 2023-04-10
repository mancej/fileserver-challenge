import dataclasses
import time
import logging


class RequestResult:
    def __init__(self, response):
        self.response = response

    def was_success(self) -> bool:
        return self.response.status_code >= 200 and self.response.status_code < 300


class ResultStats:

    def __init__(self):
        self.start_time: float = time.time()
        self.total_requests: int = 0
        self.num_success: int = 0
        self.num_failure: int = 0

    def print_stats(self):
        throughput = int(self.total_requests / (time.time() - self.start_time))
        success_throughput = int(self.num_success / (time.time() - self.start_time))
        logging.info("Test Results:")
        logging.info("------------------------------------------------")
        logging.info(f"Attempted Throughput: {throughput} requests/sec")
        logging.info(f"Successful Throughput: {success_throughput} requests/sec")
        logging.info(f"Total requests: {self.total_requests}")
        logging.info(f"Total success: {self.num_success}")
        logging.info(f"Total failure: {self.num_failure}")
        logging.info(f"\n")

    def merge(self, result: RequestResult):
        self.total_requests = self.total_requests + 1
        if result.was_success():
            self.num_success = self.num_success + 1
        else:
            self.num_failure = self.num_failure + 1
