# Binário estático para o agente rodar como microserviço dentro do Kind.
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY agent ./agent
COPY internal ./internal
COPY cmd ./cmd
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /out/k8s-mentor ./cmd/k8s-mentor

# k8sgpt empacotado junto — usado só pelo subcomando `watch` (Fase 6, execução
# periódica dentro do cluster). O agente invoca esse binário via os/exec, não
# via shell/pipe (a imagem final não tem shell), então basta o binário estar
# em algum lugar do PATH. Versão pinada pra bater com o que foi validado
# manualmente nesta sessão (ver docs/DEMO.md).
FROM ghcr.io/k8sgpt-ai/k8sgpt:v0.4.32 AS k8sgpt

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/k8s-mentor /k8s-mentor
COPY --from=k8sgpt /k8sgpt /usr/local/bin/k8sgpt
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/k8s-mentor"]
CMD ["serve"]
