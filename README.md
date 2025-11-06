# Boardgames Night Bot

Welcome to the Boardgame Night Bot project! This bot is designed to help you organize and manage board game nights with your friends.

## Features

- **Event Scheduling**: Easily schedule game nights and send invites to your friends.
- **RSVP Tracking**: Keep track of who is attending the game night.

## Installation

To install the Boardgame Night Bot, follow these steps:

1. Clone the repository:
    ```bash
    git clone https://github.com/DangerBlack/boardgame_night_bot.git
    ```
2. Navigate to the project directory:
    ```bash
    cd boardgame_night_bot
    ```
3. Create a file `.env` please include the following secrets
    ```
    TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    HEALTH_CHECK_URL=https://hc-ping.com/xxxxxxxxxxxxxxxxxxx
    BASE_URL=https://xxxxxxxxxxxxxxxxxxxxxxxxx.com
    BOT_NAME=name_of_your_bot 
    PORT=8080
    DB_PATH=./archive 
    BGG_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    ```

> [!Note]
>
> You must register MiniApp url to the bot fathers before using the bot.
>
> Please register using `/newapp` and when asked for
>
> _Now please choose a short name for your web app: 3-30 characters, `a-zA-Z0-9_`_
>
> chose `home`

## Usage

To start the bot, run the following command:

```bash
go run src/main.go
```

## Docker

```bash
docker build -t boardgames-night-bot .
docker run --env-file .env -p 8080:8080 -v ./archive:/archive boardgames-night-bot
```

## Docker Compose

```
version: "3.8"

services:
  bgg_night:
    build: .
    container_name: bgg_night
    environment:
      - TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
      - HEALTH_CHECK_URL=https://hc-ping.com/xxxxxxxxxxxxxxxxxxx
      - BASE_URL=https://xxxxxxxxxxxxxxxxxxxxxxxxx.com
      - BOT_NAME=name_of_your_bot 
      - PORT=8080
      - DB_PATH=/archive
      - BGG_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    ports:
      - "8080:8080"
    volumes:
      - ./archive:/archive
    restart: unless-stopped

```

## Contributing

We welcome contributions! Please read our [contributing guidelines](CONTRIBUTING.md) for more details.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE.md) file for details.

## Contact

If you have any questions or suggestions, feel free to open an issue or contact us.

Enjoy your game night!
