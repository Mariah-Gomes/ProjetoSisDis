# FROM python:3.11-slim

# WORKDIR /app

# COPY main.py .

# RUN pip install pyzmq

# CMD ["python", "main.py"]

FROM python:3.12-slim
RUN pip install --no-cache-dir pyzmq==26.2.0
WORKDIR /app
COPY main.py .
CMD ["python", "main.py"]
