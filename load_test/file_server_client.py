import dataclasses
import random
import base64
import time
from typing import Set

import requests
import logging
from results import RequestResult


@dataclasses.dataclass
class InvalidResponse:
    status_code: int
    text: str


class FileServerClient:
    def __init__(self, address: str, path_prefix: str, max_file_size: int):
        self.address = address
        self.path_prefix = path_prefix
        self.max_file_size = max_file_size
        self._tracked_files: Set[str] = set()
        self._in_process: Set[str] = set()  # files with current open requests.

    def wait_for_open_in_process(self, file_name: str):
        jitter = random.randint(0, 100)
        while file_name in self._in_process:
            time.sleep(0.01 * jitter)

        self._in_process.add(file_name)

    def put_file(self, file_name: str) -> RequestResult:
        self.wait_for_open_in_process(file_name)
        # Put a file
        file_size = random.randint(1, self.max_file_size)
        file_bytes = base64.b64encode(random.randbytes(file_size)).decode('ascii')
        try:
            response = requests.put(f"{self.address}/{self.path_prefix}/{file_name}", data=file_bytes)
            if 200 <= response.status_code < 300:
                self._tracked_files.add(file_name)
        except requests.exceptions.Timeout as e:
            self._in_process.remove(file_name)
            logging.error(e)
            return RequestResult(InvalidResponse(status_code=429, text=f"Server overloaded, request timeout: {e}"))
        except requests.exceptions.RequestException as e:
            self._in_process.remove(file_name)
            logging.error(e)
            return RequestResult(InvalidResponse(status_code=500, text=f"Unexpected request error: {e}"))

        self._in_process.remove(file_name)
        return RequestResult(response)

    def get_file(self, file_name: str) -> RequestResult:
        self.wait_for_open_in_process(file_name)

        # return the file contents
        logging.debug(f"GETTING file: {file_name}")
        try:
            response = requests.get(f"{self.address}/{self.path_prefix}/{file_name}")
        except requests.exceptions.Timeout as e:
            self._in_process.remove(file_name)
            logging.error(e)
            return RequestResult(InvalidResponse(status_code=429, text=f"Server overloaded, request timeout: {e}"))
        except requests.exceptions.RequestException as e:
            self._in_process.remove(file_name)
            logging.error(e)
            return RequestResult(InvalidResponse(status_code=500, text=f"Unexpected request error: {e}"))

        self._in_process.remove(file_name)
        return RequestResult(response)

    def delete_file(self, file_name: str) -> RequestResult:
        self.wait_for_open_in_process(file_name)
        # Delete the file
        logging.debug(f"DELETING file: {file_name}")
        try:
            response = requests.delete(f"{self.address}/{self.path_prefix}/{file_name}")
            if 200 <= response.status_code < 300:
                self._tracked_files.remove(file_name)
        except requests.exceptions.Timeout as e:
            logging.error(e)
            self._in_process.remove(file_name)
            return RequestResult(InvalidResponse(status_code=429, text=f"Server overloaded, request timeout: {e}"))
        except requests.exceptions.RequestException as e:
            logging.error(e)
            self._in_process.remove(file_name)
            return RequestResult(InvalidResponse(status_code=500, text=f"Unexpected request error: {e}"))

        self._in_process.remove(file_name)
        return RequestResult(response)

    # Returns a file that doesn't have an open request running. Return empty string if all files are currently in
    # process
    def get_random_not_in_process_file(self) -> str:
        possible_options = self._tracked_files.difference(self._in_process)
        if len(possible_options) == 0:
            return ""

        return random.choice(list(possible_options))

    def tracked_count(self) -> int:
        return len(self._tracked_files)

