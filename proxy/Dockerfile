FROM python:3.11-slim

WORKDIR /app

COPY main.py .

RUN pip install pyzmq

CMD ["python", "main.py"]