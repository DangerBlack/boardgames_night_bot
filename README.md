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
    BASE_URL=https://yourdomain.example
    BOT_NAME=name_of_your_bot
    BOT_MINI_APP_URL=https://t.me/botname/miniapp_name
    PORT=8080
    DB_PATH=./archive 
    BGG_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    # Optionals
    HTTP_TIMEOUT=10s 
    HTTP_MAX_ATTEMPT=3
    FAILURE_EXPIRATION=10m
    MAX_FAILURE_ATTEMPTS=5
    # Email registration (optional: without these, email sign-in is disabled)
    RESEND_API_KEY=re_xxxxxxxxxxxxxxxxxxxxxxxx
    EMAIL_FROM="Board Game Night <noreply@yourdomain.example>"
    # How long Telegram Mini App init data stays valid (default 24h)
    TELEGRAM_INIT_DATA_MAX_AGE=24h
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

## Web accounts and sharing

People can take part in events from the web, not only from Telegram:

- **Identity is verified server-side.** Inside the Telegram mini-app the page sends Telegram's signed `initData`, which the server checks with the bot `TOKEN`. Outside Telegram, visitors sign in with a one-time email link (sent through [Resend](https://resend.com)).
- **After the first sign-in by email**, the user chooses:
  - **Link Telegram** (claim): the bot deep link `/start link_<token>` proves which Telegram account is theirs. From then on they act as that Telegram user everywhere.
  - **Separate account**: they choose a username and are marked as separate from Telegram. They are shown with ✉️, and they can still link Telegram later. When they do, their participations move to the Telegram account.
- **Sharing**: every event page has a share link (`BASE_URL/events/<id>`). By default only Telegram users can join. The event owner can tick *"Let people without Telegram join"*, either when creating the event or on the event page. A 🔗 share button then also appears in the Telegram message.

Your Resend sending domain must be verified so that `EMAIL_FROM` is accepted.

## Usage

To start the bot, run the following command:

```bash
go run src/main.go
```

## Test locally

Create a forward using ngrok to port 8080 and paste the link provided into `BOT_MINI_APP_URL` .env then edit bot mini app url settings in bot father
```bash
ngrok http 8080
```

## Docker

```bash
docker build -t boardgames-night-bot .
docker run --env-file .env -p 8080:8080 -v ./archive:/archive boardgames-night-bot
```

## Docker push
```
docker buildx create --use
docker buildx build --platform linux/arm64 -t dangerblack/boardgames-night-bot:arm64 .
docker buildx build --platform linux/arm64 -t dangerblack/boardgames-night-bot:arm64 --push .
```

## Docker Compose

```
services:
  bgg_night:
    build: .
    container_name: bgg_night
    environment:
      - TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
      - HEALTH_CHECK_URL=https://hc-ping.com/xxxxxxxxxxxxxxxxxxx
      - BASE_URL=https://yourdomain.example
      - BOT_MINI_APP_URL=https://t.me/name_of_your_bot/home
      - BOT_NAME=name_of_your_bot 
      - DB_PATH=/archive
      - BGG_TOKEN=xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
    ports:
      - "8080:8080"
    volumes:
      - ./archive:/archive
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 60s
      timeout: 10s
      retries: 3
```

## Contributing

We welcome contributions! Please read our [contributing guidelines](CONTRIBUTING.md) for more details.

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE.md) file for details.

## Contact

If you have any questions or suggestions, feel free to open an issue or contact us.

Enjoy your game night!
