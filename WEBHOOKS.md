

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

- Each webhook request includes the following headers for authentication:
  - `x-ms-date`: The UTC date/time of the request (RFC1123 format).
  - `x-ms-content-sha256`: The SHA256 hash (hex-encoded) of the JSON payload.
  - `X-BGNB-Signature`: The HMAC-SHA256 signature (base64-encoded) of the string:
  
    ```javascript
    stringToSign = x-ms-date + ";" + x-ms-content-sha256
    ```

- The signature is computed as:

    1. Calculate the SHA256 hash of the JSON payload and hex-encode it (for `x-ms-content-sha256`).
    2. Get the current UTC date/time in RFC1123 format (for `x-ms-date`).
    3. Build the string to sign: `stringToSign = x-ms-date + ";" + x-ms-content-sha256`.
    4. Compute the HMAC-SHA256 of `stringToSign` using your webhook secret, then base64-encode the result (for `X-BGNB-Signature`).

- On the receiver side, verify the signature by repeating these steps and comparing the result with the `X-BGNB-Signature` header.

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
        "message_id": 123456, // nullable
        "location": "string", // nullable
        "starts_at": "YYYY-MM-DDTHH:MM:SSZ", // nullable
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
        "user_id": 123456,
        "user_name": "string",
        "name": "string",
        "max_players": 5,
        "message_id": 123456, // nullable
        "bgg": {
            "is_set": true,
            "id": 12345, // nullable
            "name": "string", // nullable
            "url": "string", // nullable
            "image_url": "string" // nullable
        },
        "created_at": "YYYY-MM-DDTHH:MM:SSZ"
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

### Update Game

When an already existing game is updated, you will receive a POST request with the following JSON body:

```json
{
    "type": "update_game",
    "data": {
        "id": 123,
        "event_id": "string",
        "user_id": 123456,
        "user_name": "string",
        "name": "string",
        "max_players": 5,
        "message_id": 123456, // nullable
        "bgg": {
            "is_set": true,
            "id": 12345, // nullable
            "name": "string", // nullable
            "url": "string", // nullable
            "image_url": "string" // nullable
        },
        "updated_at": "YYYY-MM-DDTHH:MM:SSZ"
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
        "user_name": "string",
        "added_at": "YYYY-MM-DDTHH:MM:SSZ"
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
        "user_name": "string",
        "removed_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

## Receiving Notifications

Your endpoint must accept POST requests with a JSON body.

Verify the HMAC signature using the secret sent to you by the bot.

Handle the event types as needed in your system.
