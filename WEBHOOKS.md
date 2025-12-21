

# Boardgame Night Bot Webhook Integration

You can integrate your own system with Boardgame Night Bot using webhooks. This allows you to receive real-time notifications about key events in the system.

## Registering a Webhook

To register your webhook, use the following command in the bot:

```text
/register [url]
```

Replace `[url]` with your endpoint (must be publicly accessible).

- When you register, the bot will generate a unique secret key for your webhook.
- This secret will be sent to you privately by the bot. **Keep it safe!**
- All webhook payloads will be signed using HMAC with this secret.

## Security

- Each webhook request includes an HMAC signature in the `X-BGNB-Signature` header.
- Use the secret provided to verify the authenticity and integrity of the payload.

## Webhook Events

### New Event

When a new event is created, you will receive a POST request with the following JSON body:

```json
{
    "type": "new_event",
    "data": {
        "id": "string",
        "chat_id": 123456,
        "user_id": 123456,
        "user_name": "string",
        "name": "string",
        "message_id": 123456,
        "location": "string",
        "starts_at": "YYYY-MM-DDTHH:MM:SSZ",
        "created_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### Delete Event

When an event is deleted, you will receive a POST request with the following JSON body:

```json
{
    "type": "delete_event",
    "data": {
        "id": "string",
        "name": "string",
        "user_name": "string",
        "deleted_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### New Game

When a new game is added to an event, you will receive a POST request with the following JSON body:

```json
{
    "type": "new_game",
    "data": {
        "id": 123,
        "event_id": "string",
        "name": "string",
        "max_players": 5,
        "message_id": 123456,
        "bgg": {
            "is_set": true,
            "id": 12345,
            "name": "string",
            "url": "string",
            "image_url": "string"
        }
    }
}
```

### Delete Game

When a game is deleted from an event, you will receive a POST request with the following JSON body:

```json
{
    "type": "delete_game",
    "data": {
        "event_id": "string",
        "id": 123,
        "name": "string",
        "user_name": "string",
        "deleted_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### Add Participant

When a participant is added to a game, you will receive a POST request with the following JSON body:

```json
{
    "type": "add_participant",
    "data": {
        "event_id": "string",
        "game_id": 123,
        "id": 456,
        "user_id": 789,
        "user_name": "string"
    }
}
```

### Remove Participant

When a participant is removed from a game, you will receive a POST request with the following JSON body:

```json
{
    "type": "remove_participant",
    "data": {
        "event_id": "string",
        "game_id": 123,
        "id": 456,
        "user_id": 789,
        "user_name": "string"
    }
}
```

## Receiving Notifications

Your endpoint must accept POST requests with a JSON body.

Verify the HMAC signature using the secret sent to you by the bot.

Handle the event types as needed in your system.
