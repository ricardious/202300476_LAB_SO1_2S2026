FROM python:3.12-slim-bookworm

RUN useradd --create-home --uid 10001 cpuuser

WORKDIR /opt/cpu-high
COPY cpu_high.py /opt/cpu-high/cpu_high.py

USER cpuuser

CMD ["python3", "/opt/cpu-high/cpu_high.py"]
