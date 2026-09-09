FROM python:3.12-slim-bookworm

RUN useradd --create-home --uid 10001 intruder

WORKDIR /opt/intruder
COPY intruder.py /opt/intruder/intruder.py
COPY fixture /opt/intruder/fixture

USER intruder

CMD ["python3", "/opt/intruder/intruder.py"]
