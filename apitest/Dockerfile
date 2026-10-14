FROM python:3.13-alpine
WORKDIR /work
COPY apitest/requirements.txt ./apitest/requirements.txt
RUN pip install --no-cache-dir -r apitest/requirements.txt
COPY apitest ./apitest
CMD ["python", "apitest/run_all.py"]
