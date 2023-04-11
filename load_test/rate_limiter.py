# Method that continuously rate limits (approve / deny) based on clientId and a limit of 5 msgs / sec.
# Only rate limit after a burst threshold is exceeded )

import sys
import threading
import time
import logging
from dataclasses import dataclass
from typing import Dict

root = logging.getLogger()
root.setLevel(logging.INFO)
root.addHandler(logging.StreamHandler(sys.stdout))

DEFAULT_MSG_PER_SEC = 5
BURST_BALANCE = 100
BURST_BALANCE_RELOAD_RATE = 60 * 60 * 1000  # 1 hour
NEVER_UPDATE_BURST_BALANCE = 60 * 60 * 1000 * 24 * 365 * 10  # 10 years


class Utils:
    @staticmethod
    def now_millis():
        return int(time.time() * 1000)


class LockableSingleton:
    _instance = None
    lock = threading.Lock()

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            with cls.lock:
                # another thread could have created the instance
                # before we acquired the lock. So check that the
                # instance is still nonexistent.
                if not cls._instance:
                    cls._instance = super(LockableSingleton, cls).__new__(cls)
        return cls._instance



@dataclass
class RateLimitedClient:
    client_id: str
    burst_balance: int = BURST_BALANCE
    request_balance: int = DEFAULT_MSG_PER_SEC
    last_request_balance_update: int = Utils.now_millis()
    last_burst_balance_update = Utils.now_millis()
    message_per_sec: int = DEFAULT_MSG_PER_SEC
    burst_balance_reload_interval: int = NEVER_UPDATE_BURST_BALANCE
    burst_balance_maximum: int = BURST_BALANCE

    def _hydrate_balance(self):
        # add new accrued requests since last call
        request_balance_addition = int(
            (Utils.now_millis() - self.last_request_balance_update) / 1000 * self.message_per_sec)
        burst_balance_addition = 0

        if request_balance_addition:
            self.request_balance = min(self.request_balance + request_balance_addition, self.message_per_sec)
            self.last_request_balance_update = Utils.now_millis()

        if self.burst_balance_maximum > 0:
            burst_balance_addition = int((Utils.now_millis() - self.last_burst_balance_update) * self.burst_balance_maximum / self.burst_balance_reload_interval)
            self.burst_balance = min(self.burst_balance + burst_balance_addition, self.burst_balance_maximum)
            self.last_burst_balance_update = Utils.now_millis()

        logging.debug(f"Adding request balance: {request_balance_addition} and burst_balance: {burst_balance_addition}")

    def is_allowed(self):
        self._hydrate_balance()

        if self.request_balance > 0:
            self.request_balance = self.request_balance - 1
            logging.debug(f"Request allowed for client: {self.client_id}, remaining balance: {self.request_balance}")
            return True
        elif self.burst_balance > 0:
            self.burst_balance = self.burst_balance - 1
            logging.debug(f"BURST USED for client: {self.client_id}, remaining burst: {self.burst_balance}")
            return True
        else:
            logging.debug(f"{self.client_id} THROTTLED!")
            return False


# Performs client-side rate limiting so the load test can target a particular req/sec throughput.
class RateLimiter(LockableSingleton):
    def __init__(self, throughput_per_second: int = 0, burst_balance_maximum: int = 0,
                 burst_balance_reload_interval: int = NEVER_UPDATE_BURST_BALANCE):
        self.burst_balance_maximum = burst_balance_maximum
        self.burst_balance_reload_interval = burst_balance_reload_interval
        self.throughput_per_second = throughput_per_second
        self.limited_clients: Dict[str: RateLimitedClient] = {}

    def is_allowed(self, client_id: str) -> bool:
        with self.lock:
            if client_id not in self.limited_clients.keys():
                self.limited_clients[client_id] = RateLimitedClient(client_id=client_id,
                                                                    message_per_sec=self.throughput_per_second,
                                                                    burst_balance_reload_interval=self.burst_balance_reload_interval,
                                                                    burst_balance_maximum=self.burst_balance_maximum,
                                                                    burst_balance=0)

            return self.limited_clients[client_id].is_allowed()

    def get_clients(self) -> Dict:
        return self.limited_clients

    def log_stats(self):
        with self.lock:
            for client_id, rlc in self.limited_clients.items():
                logging.debug(
                    f"Client: {rlc.client_id} : Req Balance: {rlc.request_balance}, Burst Balance: {rlc.burst_balance}")
