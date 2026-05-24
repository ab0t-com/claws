How to play with it — the actual run-through
  
  You'll need ~10 minutes total: 5 for the bot token, 5 for OpenAI key, then ~30s to apply.

  Step 1 — Get a Telegram bot token (~5 min, free, no card)

  On your phone or laptop:

  1. Open Telegram. Search for @BotFather (the verified blue-tick one).
  2. Tap Start.
  3. Send: /newbot
  4. BotFather asks for a friendly name — type anything, e.g. Mike's Test Claw.
  5. BotFather asks for a username — must end in bot and be unique. E.g. mikes_test_claw_bot.
  6. BotFather replies with a token of the form `<digits>:<long-string>`
     (about 46 characters total — looks like `<bot-id>:<secret>`)
  6. Copy that whole line (digits + colon + the long string).
  7. Bonus: BotFather also gives you a link like t.me/mikes_test_claw_bot — that's the URL you'll DM later.

  Step 2 — Get an OpenAI API key (~5 min)

  1. Go to https://platform.openai.com/api-keys in a browser.
  2. Sign in (or sign up — needs payment method but you get free credits).
  3. Click + Create new secret key → name it claws-demo → Create.
  4. Copy the key (starts with sk-proj-...). It's shown ONCE — paste it somewhere you won't lose it.

  Step 3 — On your server (this host, since claws is already installed here)

  cd ~/claw/workspace/clawctl-go

  # Initialise the secrets dir
  bash scripts/setup-secrets.sh
  # → creates /tmp/claws-secrets/ with 9 placeholder files

  # Paste the two tokens
  nano /tmp/claws-secrets/openai.key       # paste the sk-proj-... key on its own line
  nano /tmp/claws-secrets/telegram.token   # paste the <bot-id>:<secret> token on its own line
  # (lines starting with # are stripped — just paste the value)

  Step 4 — Pick solo or team

  Solo bot (DM one agent, it replies):

  ./claws apply --template=demo/instant-bot --secrets-dir=/tmp/claws-secrets
  ./claws start default/instant

  Team (DM a coordinator, it delegates to a worker, relays the answer):

  ./claws apply --template=demo/instant-team --secrets-dir=/tmp/claws-secrets
  ./claws start instant-team/coordinator
  ./claws start instant-team/worker

  Step 5 — DM your bot

  Open the t.me/your_bot link from BotFather. Tap Start. Type any message. The agent replies.

  Step 6 — Verify if anything's off

  # Solo
  ./claws agent ping default/instant

  # Team
  ./claws agent ping instant-team/coordinator
  ./claws agent ping instant-team/worker

  # Or: one screen for everything
  ./claws fleet doctor

  ---
  What to expect on first DM
  
  - First message reply: ~2-5 seconds (model cold-start).
  - DM pairing: the agent might ask you for a pairing code on first contact (security default in v1.6.0). If so, the token /start reply will tell you what to do — usually /pair followed by a short code.
  - If silent: ./claws logs default/instant -f shows what's happening. Common causes:
    - Wrong OpenAI key → claws agent ping flags "auth not configured"
    - Wrong bot token → no error visible client-side; check logs
    - Free OpenAI credits exhausted → 429 errors in logs

  ---
  Tear-down when done

  ./claws stop default/instant       # or instant-team/coordinator, instant-team/worker
  ./claws remove default/instant --purge   # full delete; or just stop and leave for next time
  trash /tmp/claws-secrets/          # if you want the tokens wiped (or leave for next session)

  Then in BotFather: /deletebot → pick your bot → confirm. (Or keep it — it's free and you can re-use the token next time.)
