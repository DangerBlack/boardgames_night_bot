

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

> **Note:** Webhook communication is **bi-directional**. The Boardgame Night Bot both listens for these events (as a receiver) and dispatches them (as an emitter). Your system can send supported events to the bot, and will also receive these events from the bot.

### New Event

This JSON payload describe the action of create a new event, is dispatched when an event is created in the system and can be received to create a new event.

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

This JSON payload describe the action of delete an event, this event exists only for incoming webhooks and can be received to delete an event.

```json
{
    "type": "delete_event",
    "data": {
        "event_id": "string",
        "user_id": 123456,
        "user_name": "string",
        "deleted_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### New Game

This JSON payload describe the action of add a new game to an event, is dispatched when a game is added to an event in the system and can be received to add a game to an event.

```json
{
    "type": "new_game",
    "data": {
        "id": "string",
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

This JSON payload describe the action of delete a game from an event, is dispatched when a game is deleted in the system and can be received to delete a game from an event.

```json
{
    "type": "delete_game",
    "data": {
        "event_id": "string",
        "id": "string",
        "name": "string",
        "user_id": 123456,
        "user_name": "string",
        "deleted_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### Update Game

This JSON payload describe the action of updating an existing game, is dispatched when a game is updated in the system and can be received to update a game.

```json
{
    "type": "update_game",
    "data": {
        "id": "string",
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

This JSON payload describe the action of adding a participant to a game, is dispatched when a participant is added in the system and can be received to add a participant. A participant can only join a single game, adding a participant to game B remove it from game A on the same event ID.

```json
{
    "type": "add_participant",
    "data": {
        "id": "string",
        "event_id": "string",
        "user_id": 789,
        "game_id": "string",
        "user_name": "string",
        "added_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### Remove Participant

This JSON payload describe the action of removing a participant from an event, is dispatched when a participant is removed from an event in the system and can be received to remove a participant.

```json
{
    "type": "remove_participant",
    "data": {
        "id": "string",
        "event_id": "string",
        "user_id": 789,
        "game_id": "string",
        "user_name": "string",
        "removed_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### Send Message

Use this to send message to the chat where the webhooks is associated to:

```json
{
    "type": "send_message",
    "data": {
        "id": "string",
        "user_id": 123456,
        "user_name": "string",
        "message": "string",
        "sent_at": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```

### Test

Use this to test if the webhooks is properly configured. You can also run the special command `/test` to call the webhooks.

```json
{
    "type": "test",
    "data": {
        "message": "Webhook test successful",
        "timestamp": "YYYY-MM-DDTHH:MM:SSZ"
    }
}
```


## Receiving Notifications

Your endpoint must accept POST requests with a JSON body, is expected to return 2xx response code, 200 is recommended.

Verify the HMAC signature using the secret sent to you by the bot.

Handle the event types as needed in your system.


## Javascript example sending request

Below is a practical implementation showing how to cryptographically sign a send_message webhook event.
The example generates the required UTC timestamp, hashes the payload using SHA256, builds the signing string, and produces an HMAC SHA256 signature encoded in base64. This can be used in a Node.js script or REPL before dispatching the webhook request.

```javascript

// Example: signing a send_message webhook payload using HMAC SHA256.
// This snippet shows how to compute the required headers and signature
// before sending the webhook request from a Node.js REPL or script.

const crypto = require("crypto");

const webhookSecret = "YOUR_WEBHOOK_SECRET";
const webhookRegisteredURL = "YOUR_REGISTERED_WEBHOOK_URL";

const payload = JSON.stringify({
  type: "send_message",
  data: {
    id: "msg_001",
    user_id: 42,
    user_name: "Elia",
    message: "Game night starts soon!",
    sent_at: "2025-12-26T18:30:00Z"
  }
});

const payloadHash = crypto.createHash("sha256").update(payload).digest("hex");

const msDate = new Date().toUTCString();

const stringToSign = msDate + ";" + payloadHash;

const signature = crypto
  .createHmac("sha256", webhookSecret)
  .update(stringToSign)
  .digest("base64");

console.log("x-ms-date:", msDate);
console.log("x-ms-content-sha256:", payloadHash);
console.log("x-bgnb-signature:", signature);

fetch(webhookRegisteredURL, {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
    "x-ms-date": msDate,
    "x-ms-content-sha256": payloadHash,
    "x-bgnb-signature": signature
  },
  body: payload
}).then(console.log).catch(console.error);
```

## Javascript example registering a webhook in express

To receive webhook events, create a POST route in Express and attach the signature verification middleware. The route should parse JSON and return a 2xx response when the signature and timestamp are valid.

```
const crypto = require("crypto");

function verifyBGNBSignature(secret, maxAgeSeconds = 300) {
  return function (req, res, next) {
    const dateHeader = req.headers["x-ms-date"];
    const receivedHash = req.headers["x-ms-content-sha256"];
    const receivedSig = req.headers["x-bgnb-signature"];

    if (!dateHeader || !receivedHash || !receivedSig) {
      return res.status(401).json({ error: "Missing auth headers" });
    }

    const requestTime = Date.parse(dateHeader);
    const now = Date.now();
    const ageSeconds = (now - requestTime) / 1000;

    if (isNaN(requestTime) || ageSeconds > maxAgeSeconds) {
      return res.status(403).json({ error: "Request expired or timestamp invalid" });
    }

    const rawBody = JSON.stringify(req.body);
    const computedHash = crypto.createHash("sha256").update(rawBody).digest("hex");

    if (computedHash !== receivedHash) {
      return res.status(401).json({ error: "Invalid payload hash" });
    }

    const toSign = dateHeader + ";" + computedHash;
    const computedSig = crypto.createHmac("sha256", secret).update(toSign).digest("base64");

    if (computedSig !== receivedSig) {
      return res.status(401).json({ error: "Invalid signature" });
    }

    next();
  };
}

module.exports = verifyBGNBSignature;
```

```
const express = require("express");
const verifyBGNBSignature = require("./verifyBGNBSignature");

const app = express();
app.use(express.json());

app.post("/webhook", verifyBGNBSignature("YOUR_WEBHOOK_SECRET"), (req, res) => {
  console.log(req.body);
  res.status(200).json({ ok: true, type: req.body.type });
});

app.listen(3000, () => console.log("Webhook server ready"));
```

Now register the listener to your chat using

`register https://localhost:3000/webhook`
