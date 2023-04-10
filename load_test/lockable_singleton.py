import threading


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
