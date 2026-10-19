import json, os, random, uuid, itertools
from datetime import datetime, timezone
from locust import HttpUser, task, between, events
from pathlib import Path
profile = json.loads(Path(os.environ.get("SO1_LOAD_PROFILE", Path(__file__).with_name("load-profile.json"))).read_text())
report_path = os.environ.get("SO1_LOAD_PATH", "/grpc-202300476")
if report_path not in ("/grpc-202300476", "/dapr-202300476"):
    raise ValueError("SO1_LOAD_PATH debe seleccionar grpc o dapr del carnet 202300476")
@events.init_command_line_parser.add_listener
def select_report_path(parser):
    parser.add_argument("--report-path", choices=["/grpc-202300476", "/dapr-202300476"], default=report_path, help="Ruta publica: flujo gRPC o Dapr", include_in_web_ui=True)
user_ids = itertools.count()
class Reports(HttpUser):
    wait_time = between(float(os.environ.get("SO1_LOAD_WAIT_MIN", "0.5")), float(os.environ.get("SO1_LOAD_WAIT_MAX", "1.5")))
    def on_start(self):
        self.rng = random.Random(profile["seed"] + next(user_ids))
    @task
    def report(self):
        data = {"country": self.rng.choice(profile["countries"]), "warplanes_in_air": self.rng.randint(*profile["planes"]), "warships_in_water": self.rng.randint(*profile["ships"]), "timestamp": datetime.now(timezone.utc).isoformat()}
        path = self.environment.parsed_options.report_path
        self.client.post(path, json=data, headers={"Idempotency-Key": str(uuid.uuid4())}, name=path)
