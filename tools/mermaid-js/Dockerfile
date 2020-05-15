# based on https://github.com/mermaid-js/mermaid.cli/blob/33a8334293308af13d307d5c3754841636e46da8/Dockerfile
FROM node:14.2.0-stretch

WORKDIR /mermaid-js

RUN apt-get update && \
    apt-get install -y gconf-service libxext6 libxfixes3 libxi6 libxrandr2 libxrender1 libcairo2 libcups2 libdbus-1-3 libexpat1 libfontconfig1 libgcc1 libgconf-2-4 libgdk-pixbuf2.0-0 libglib2.0-0 libgtk-3-0 libnspr4 libpango-1.0-0 libpangocairo-1.0-0 libstdc++6 libx11-6 libx11-xcb1 libxcb1 libxcomposite1 libxcursor1 libxdamage1 libxss1 libxtst6 libappindicator1 libnss3 libasound2 libatk1.0-0 libc6 ca-certificates fonts-liberation lsb-release xdg-utils wget &&\
    npm install @mermaid-js/mermaid-cli@8.4.8

ADD puppeteer-config.json ./puppeteer-config.json

WORKDIR /data

ENTRYPOINT ["/mermaid-js/node_modules/.bin/mmdc", "-p", "/mermaid-js/puppeteer-config.json"]
CMD [ "--help" ]
