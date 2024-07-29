docker build -t noy/mytest:latest .
cd deploy || exit
docker-compose up -d
