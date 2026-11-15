// Package observability centraliza a integração com o New Relic (APM +
// Kubernetes), escolhida entre Datadog/New Relic conforme o requisito de
// Monitoramento e Observabilidade da Fase 3 (ver 13SOAT Fase 3 Tech
// Challenge.pdf). New Relic foi escolhido pelo tier gratuito permanente
// (sem necessidade de cartão de crédito) e por não expirar como um trial.
package observability

import (
	"log"
	"os"

	"github.com/newrelic/go-agent/v3/newrelic"
)

// NewRelicApp inicializa o agente do New Relic a partir de variáveis de
// ambiente (NEW_RELIC_LICENSE_KEY, NEW_RELIC_APP_NAME). Se a license key não
// estiver configurada — como em testes locais/CI que não têm conta no New
// Relic — devolve nil sem erro: o restante do código trata "app == nil"
// como "observabilidade desligada, segue o fluxo normalmente", em vez de
// exigir a chave pra rodar a aplicação (diferente do Postgres, que é uma
// dependência obrigatória).
func NewRelicApp() *newrelic.Application {
	licenseKey := os.Getenv("NEW_RELIC_LICENSE_KEY")
	if licenseKey == "" {
		log.Println("NEW_RELIC_LICENSE_KEY não configurada — observabilidade via New Relic desligada")
		return nil
	}

	appName := os.Getenv("NEW_RELIC_APP_NAME")
	if appName == "" {
		appName = "autoshop-execution"
	}

	opts := []newrelic.ConfigOption{
		newrelic.ConfigAppName(appName),
		newrelic.ConfigLicense(licenseKey),
		newrelic.ConfigAppLogForwardingEnabled(true),
		newrelic.ConfigDistributedTracerEnabled(true),
		newrelic.ConfigFromEnvironment(),
	}

	// Diferente de outros agentes New Relic (Node, Python, Java), o agente Go
	// NÃO lê NEW_RELIC_LOG/NEW_RELIC_LOG_LEVEL como variável de ambiente —
	// o log de diagnóstico do agente (harvest, conexão com o collector, erros
	// de rede) só liga via ConfigDebugLogger/ConfigInfoLogger no código. Ativa
	// aqui com NEW_RELIC_DEBUG=true no .env, só pra depurar problema de
	// conectividade — deixa desligado (padrão) no dia a dia.
	if os.Getenv("NEW_RELIC_DEBUG") == "true" {
		opts = append(opts, newrelic.ConfigDebugLogger(os.Stdout))
	}

	app, err := newrelic.NewApplication(opts...)
	if err != nil {
		// Erro de configuração do agente não deve derrubar a API — loga e
		// segue sem observabilidade, igual ao caso de chave ausente.
		log.Printf("erro ao inicializar o agente do New Relic (observabilidade desligada): %v", err)
		return nil
	}

	log.Printf("New Relic APM habilitado — app: %s", appName)
	return app
}
