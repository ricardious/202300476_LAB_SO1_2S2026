FROM python:3.12-slim-bookworm

RUN useradd --create-home --uid 10001 lowuser

WORKDIR /opt/low
COPY low.py /opt/low/low.py

USER lowuser

CMD ["python3", "/opt/low/low.py"]
