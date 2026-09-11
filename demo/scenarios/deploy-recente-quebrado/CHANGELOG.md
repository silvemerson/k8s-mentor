# "git log" simulado do payment-worker

Não é um repositório de verdade — é só o texto que a Fase 3 (correlação
GitOps) vai eventualmente ler/parsear de um `git log` real. Documentado aqui
pra já deixar claro o formato esperado e servir de dado de apoio na demo
enquanto essa fase não existe.

```
e4f5a6b (HEAD) fix: normaliza nome das env vars do payment-worker
a1b2c3d chore: bump payment-worker to v1.4.0
9f0e1d2 feat: adiciona payment-worker
```

Na demo: aplique `v1-ok.yaml`, espere ficar `Available`, só então aplique
`v2-broken.yaml` (é o que `apply.sh` deste cenário faz) — assim existe um
intervalo de tempo real entre os dois estados, não só na aparência.
