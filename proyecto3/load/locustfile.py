import json, os, random, uuid, itertools
from datetime import datetime, timezone
from locust import HttpUser, task, between
from pathlib import Path
profile = json.loads(Path(os.environ.get("SO1_LOAD_PROFILE", Path(__file__).with_name("load-profile.json"))).read_text())
user_ids = itertools.count()
class Reports(HttpUser):
    wait_time = between(float(os.environ.get("SO1_LOAD_WAIT_MIN", "0.5")), float(os.environ.get("SO1_LOAD_WAIT_MAX", "1.5")))
    def on_start(self):
        self.rng = random.Random(profile["seed"] + next(user_ids))
    @task
    def report(self):
        data = {"country": self.rng.choice(profile["countries"]), "warplanes_in_air": self.rng.randint(*profile["planes"]), "warships_in_water": self.rng.randint(*profile["ships"]), "timestamp": datetime.now(timezone.utc).isoformat()}
        self.client.post("/grpc-202300476", json=data, headers={"Idempotency-Key": str(uuid.uuid4())}, name="military-report")
