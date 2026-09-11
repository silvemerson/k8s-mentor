# Runbook: OOMKilled recorrente no checkout-service

## Sintomas

O pod do `checkout-service` reinicia com `OOMKilled` (exit code 137) de forma
cíclica, geralmente a cada poucas horas ou dias. O padrão característico:
memória sobe de forma aproximadamente linear desde o boot do processo até
estourar o `limits.memory` — não é um pico repentino, é crescimento contínuo.
Aumentar o limite de memória adia o problema, mas ele volta a acontecer depois
de um tempo proporcionalmente maior.

## Causa raiz

O `checkout-service` mantém um cache de sessões de carrinho em memória
(`internal/cart/session_cache.go`) que nunca expira entradas antigas — cada
requisição de checkout adiciona uma entrada ao cache, mas nada a remove
depois que a sessão termina. Em produção, isso se manifesta como um
vazamento de memória: o processo cresce continuamente com o tráfego
acumulado, até bater no limite do container.

## Como confirmar que é esse problema

- O `OOMKilled` acontece de novo mesmo depois de aumentar `limits.memory`
  (só muda o tempo até acontecer de novo, não elimina o problema).
- A métrica de memória do processo (RSS) cresce de forma aproximadamente
  linear entre reinícios, sem platô — isso é o principal diferencial de um
  vazamento real vs. um limite genuinamente subdimensionado (que geralmente
  se estabiliza perto do limite).
- O histórico de incidentes mostra múltiplas ocorrências no mesmo namespace,
  cada uma "resolvida" só com aumento de limite, nunca com fix de causa raiz.

## Correção

Fix definitivo: `session_cache.go` precisa de um TTL nas entradas do cache
(ou um LRU com tamanho máximo). Ticket de tech debt: aguardando priorização
do time de checkout — enquanto isso, o workaround paliativo é reiniciar o
deployment manualmente a cada ~48h via `kubectl rollout restart` em horário
de baixo tráfego, o que evita o `OOMKilled` "surpresa" durante pico.

## Prevenção

Depois do fix de causa raiz, adicionar um alerta de tendência (não só de
threshold) sobre a métrica de memória do `checkout-service` — crescimento
sustentado por mais de N horas sem platô deve disparar alerta antes do
OOMKilled acontecer, não depois.
