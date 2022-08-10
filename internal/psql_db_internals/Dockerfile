FROM rav28/curator-modified-psql:v0.0.1

# Install python/pip
ENV PYTHONUNBUFFERED=1
RUN apk add --update --no-cache python3 && ln -sf python3 /usr/bin/python
RUN python3 -m ensurepip
RUN pip3 install --no-cache --upgrade pip setuptools

COPY email_alert.py ./
COPY execute_py.sh /docker-entrypoint-initdb.d/