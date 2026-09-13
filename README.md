# ventago-homelab

Monorepo Go del orquestador del home lab Tecodigi. Un binario ligero despacha
los workers (monitor New Relic, digest matutino, OCR y los que se agreguen).
No es VentaGO productivo ni el VPS.

Workers de negocio: **stubs deshabilitados**. Infraestructura inicial: config,
HTTP `/health`, Docker y deploy local por SSH.

## Repo

- GitHub: `Codi-Devs/ventago-homelab` (publico)
- Rama: `main` (no hay `live`)
- Knowledge: `.agents/knowledge/homelab.md` en el cerebro Tecodigi

## Layout

```
cmd/orchestrator/     binario
internal/config/      JSON, modelo qwen3.5:4b
internal/workers/     stubs nr_monitor, morning_digest, ocr
internal/orchestrator HTTP + loop de despacho
configs/              config.json (sin secretos)
scripts/deploy.sh     pipeline local → laptop
```

## Deploy (local, no GitHub Actions)

El script **no** habla SSH directo. Usa
`.agents/scripts/homelab_ssh.sh` del clone de knowledge (carpeta padre, o
`VENTAGO_KNOWLEDGE`).

```sh
go test ./...
./scripts/deploy.sh
```

Destino: `/home/ventago/homelab/orchestrator` en `192.168.40.99`.
Compose se une a la red `ventago-lab_default` (Ollama/PaddleOCR). Health en
`127.0.0.1:8080` de la laptop (no en LAN).

Rollback:

```sh
# desde el cerebro
.agents/scripts/homelab_ssh.sh exec -- 'cd /home/ventago/homelab/orchestrator && docker compose down'
```

## Reglas

- LLM: `qwen3.5:4b` por API de Ollama. No Open WebUI. No `deepseek-r1:8b`.
- No publicar puertos en `0.0.0.0`.
- No copiar secretos al repo. `.env` local esta en `.gitignore`.
