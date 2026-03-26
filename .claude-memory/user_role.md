---
name: user-role
description: User is building an AI agent orchestration platform, thinks like a product owner and architect simultaneously
type: user
---

The user is building clawctl as a product — not just writing code. They think in terms of:
- PMM perspective: user journeys, onboarding, market positioning
- Principal architect perspective: security boundaries, adapter patterns, control planes
- Operator perspective: "our system needs to do everything needed" — no manual steps

They run a team of AI agents (sarah, john, lead) on a real server, connected to real messaging channels (WhatsApp, Telegram). This is production use, not a toy.

They expect clawctl to handle full lifecycle — if `group add` moves an instance, it should stop/start containers too. If a channel is added, one command should do config + restart + tell the user what's next. No loose ends.
