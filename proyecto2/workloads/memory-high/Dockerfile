FROM python:3.12-slim-bookworm

RUN useradd --create-home --uid 10001 memuser

WORKDIR /opt/memory-high
COPY memory_high.py /opt/memory-high/memory_high.py

USER memuser

CMD ["python3", "/opt/memory-high/memory_high.py"]
