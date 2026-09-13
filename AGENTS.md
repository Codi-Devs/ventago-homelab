# ventago-homelab

Orquestador del home lab Tecodigi. Playbook y SSH: el cerebro
(`.agents/knowledge/homelab.md`, skills `homelab-connect` / `homelab-dev` /
`homelab-deploy`).

- Conexion al host: solo `.agents/scripts/homelab_ssh.sh`. Nunca `ssh` directo.
- Deploy: `./scripts/deploy.sh` (local). No GitHub Actions. No rama `live`.
- LLM: `qwen3.5:4b` por API. No Open WebUI. No `deepseek-r1:8b`.
- No implementar monitor/digest/OCR en este esqueleto; workers son stubs.
