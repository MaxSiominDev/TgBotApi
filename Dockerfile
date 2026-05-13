FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    libssl3 \
    zlib1g \
    && rm -rf /var/lib/apt/lists/*

COPY telegram-bot-api /usr/local/bin/telegram-bot-api
COPY entrypoint.sh /entrypoint.sh

RUN chmod +x /usr/local/bin/telegram-bot-api /entrypoint.sh

VOLUME ["/data"]
EXPOSE 8081

ENTRYPOINT ["/entrypoint.sh"]
