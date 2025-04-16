SERVICE=twitchannouncer

up:
	docker-compose up --build -d

stop:
	docker-compose stop

down:
	docker-compose down

restart: stop up

logs:
	docker-compose logs -f $(SERVICE)

sh:
	docker exec -it $(SERVICE) /bin/sh
