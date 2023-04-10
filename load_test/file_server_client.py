import random
import base64
import requests
import logging
from results import RequestResult


class FileServerClient():
    def __init__(self, address: str, path_prefix: str, max_file_size: int):
        self.address = address
        self.path_prefix = path_prefix
        self.max_file_size = max_file_size
    def put_file(self, file_name: str) -> RequestResult:
        # Put a file
        file_size = random.randint(1, self.max_file_size)
        file_bytes = base64.b64encode(random.randbytes(file_size)).decode('ascii')

        response = requests.put(f"{self.address}/{self.path_prefix}/{file_name}", data=file_bytes)
        logging.debug(f"PUTTING file: {file_name} with data: {file_bytes}")

        return RequestResult(response)


    def get_file(self, file_name: str) -> RequestResult:
        # return the file contents
        logging.debug(f"GETTING file: {file_name}")
        response = requests.get(f"{self.address}/{self.path_prefix}/{file_name}")

        return RequestResult(response)


    def delete_file(self, file_name: str) -> RequestResult:
        # Delete the file
        logging.debug(f"DELETING file: {file_name}")
        response = requests.delete(f"{self.address}/{self.path_prefix}/{file_name}")
        return RequestResult(response)
