<!-- GSD:project-start source:PROJECT.md -->
## Project

**goinvest**

Ferramenta de terminal (TUI, escrita em Go) para análise fundamentalista de ativos brasileiros — ações e FIIs na v1 (ETFs/BDRs em v2). Ela coleta os indicadores que hoje o usuário levanta na mão abrindo várias abas do Fundamentus, compara ativos lado a lado (tipicamente pares do mesmo setor) e **explica o que cada número significa** enquanto mostra os dados.

É uma ferramenta pessoal, de uso individual, feita para um investidor iniciante que quer decidir melhor *e* aprender no processo — não um produto multiusuário nem um robô de recomendação.

**Core Value:** Transformar uma tarde de comparação manual em dois minutos de terminal — sem que o usuário precise já saber ler os indicadores, porque a ferramenta ensina enquanto compara.

### Constraints

- **Tech stack**: Go — escolha do usuário, e boa para binário único, CLI/TUI e concorrência na coleta
- **Interface**: TUI interativa; usuário sugeriu Bubble Tea (charmbracelet) — ecossistema Charm é o padrão de fato para TUI em Go
- **Custo**: fontes de dados gratuitas — o Fundamentus foi escolhido pelo usuário por ser grátis; APIs pagas contrariam a premissa do projeto
- **Dependências**: nenhuma fonte oferece contrato estável de API pública para os dados desejados; a coleta é inerentemente frágil e o design precisa absorver isso (cache local + fontes plugáveis + degradação explícita)
- **Segurança**: nenhuma credencial de corretora ou da B3 armazenada — restrição derivada da decisão de não automatizar login
- **Escala**: uso local, mono-usuário, execução sob demanda — não há requisito de throughput, latência de rede ou disponibilidade
- **Ética/legal**: raspagem deve ser respeitosa (rate limiting, identificação, cache agressivo) para não sobrecarregar sites gratuitos de terceiros
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## TL;DR — as cinco decisões que importam
## Recommended Stack
### Core Technologies
| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| **Go** | 1.25+ (toolchain 1.26.x) | Linguagem | Já é constraint do projeto. `go 1.25.0` é o mínimo exigido pelo Charm v2 e pelo bubble-table |
| **`charm.land/bubbletea/v2`** | **v2.0.9** | Framework TUI (Elm/MVU) | Padrão de fato para TUI em Go. v2 é **GA desde ~mar/2026** (v2.0.2), hoje em patches de estabilidade. Ecossistema (bubbles, lipgloss, glamour, fang) todo alinhado em v2 |
| **`charm.land/bubbles/v2`** | **v2.2.1** | Componentes: `help`, `viewport`, `textinput`, `list`, `spinner`, `key`, `paginator`, `tree` | Componentes oficiais. `help` resolve o rodapé de atalhos; `viewport` resolve o painel de detalhe/glossário rolável; `key` dá keymap declarativo que alimenta o help automaticamente |
| **`charm.land/lipgloss/v2`** | **v2.0.6** | Layout, cores, bordas, `table` de renderização | Camada de estilo do Charm. `lipgloss/v2/table` com `StyleFunc(row, col)` é o caminho para tabelas **estáticas** com cor por célula |
| **`github.com/evertras/bubble-table`** | **v0.22.3** | **Tabela interativa ordenável/filtrável** | Já em Charm v2 (`charm.land/bubbletea/v2 v2.0.5`). Entrega pronto: `SortByAsc/Desc` + `ThenSortBy*` (multi-coluna, numérico-aware), `WithFuzzyFilter`, `NewStyledCell` (semáforo por célula), `WithHorizontalFreezeColumnCount` (trava o ticker ao rolar indicadores), `WithMissingDataIndicator` (dado ausente), paginação |
| **`modernc.org/sqlite`** | **v1.58.0** (SQLite 3.53.4) | Persistência local + histórico | **Puro Go, zero cgo.** Binário 100% estático, cross-compile trivial. Único requisito duro: binário único distribuível |
| **`github.com/PuerkitoBio/goquery`** | **v1.13.0** | Parsing HTML (seletores tipo jQuery) | O HTML do Fundamentus é `<td class="label">`/`<td class="data">` — seletores CSS resolvem direto. Sem JS na página, sem headless browser |
| **`golang.org/x/net`** | **v0.58.0** | `html/charset` — transcodificação ISO-8859-1 → UTF-8 | **Obrigatório.** Ver seção "Encoding" abaixo |
| **`github.com/xuri/excelize/v2`** | **v2.11.0** | Ler `.xlsx` da B3 | Padrão de fato em Go para Excel. Puro Go (verificado com `CGO_ENABLED=0`), mantido ativamente, lê `.xlsx`/`.xlsm` |
| **`github.com/spf13/cobra`** | **v1.10.2** | Estrutura de comandos CLI | Padrão do ecossistema. Subcomandos (`compare`, `import`, `sync`, `show`) + flags + completions |
### Supporting Libraries
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `charm.land/fang` | v1.0.0 | Wrapper de `cobra`: help/erros estilizados, `--version` do build info, manpages, completions | Envolver o root command. ~5 linhas, ganho estético grande e consistente com a TUI |
| `charm.land/glamour` | v1.0.0 | Renderizar Markdown no terminal | **Camada pedagógica**: escreva o glossário em Markdown (`//go:embed glossario/*.md`) e renderize no painel de detalhe. Separa conteúdo didático de código |
| `golang.org/x/time` | v0.15.0 | `rate.Limiter` | Rate limiting respeitoso na coleta. Um `*rate.Limiter` por host, ex. `rate.NewLimiter(rate.Every(2*time.Second), 1)` |
| `github.com/pressly/goose/v3` | v3.28.0 | Migrações SQL versionadas | Com `embed.FS` + `goose.SetDialect("sqlite3")`. Verificado funcionando sobre `modernc.org/sqlite` |
| `github.com/adrg/xdg` | v0.5.3 | Caminhos XDG cross-platform | `xdg.ConfigFile("goinvest/config.toml")`, `xdg.DataFile("goinvest/goinvest.db")`, `xdg.CacheFile(...)`. Trata Linux/macOS/Windows corretamente |
| `github.com/BurntSushi/toml` | v1.6.0 | Config file | TOML é legível para humano editar à mão (pesos da nota agregada, watchlist, tickers). ~1 dependência |
| `golang.org/x/text` | v0.41.0 | Encodings, collation | Puxado por `x/net/html/charset`. Útil para ordenação alfabética correta em pt-BR |
| `github.com/shopspring/decimal` | v1.4.0 | Decimal exato | **Só para valores monetários da carteira** (quantidade × preço, custo médio, renda). Indicadores derivados (P/L, DY) podem ser `float64` |
| `github.com/cenkalti/backoff/v5` | v5.0.3 | Retry com backoff exponencial | Retry de rede. API v5 é context-first e mais limpa que a v4 |
| `github.com/dustin/go-humanize` | v1.0.1 | "há 3 dias", "1,2 mi" | Indicador de **frescor do dado** ("atualizado há 2 dias") — requisito explícito do PROJECT.md |
### Development Tools
| Tool | Purpose | Notes |
|------|---------|-------|
| `github.com/charmbracelet/x/exp/teatest/v2` | Teste de TUI com golden files | **Verificado rodando contra bubbletea v2.0.9.** `NewTestModel` + `WithInitialTermSize` + `WaitFor` + `RequireEqualOutput`. Atualize golden com `go test -update` |
| `net/http/httptest` (stdlib) | Servir fixtures HTML gravadas | Base da estratégia de teste de scraper. Ver seção "Testes" |
| `github.com/google/go-cmp` | Diffs legíveis em asserts | `cmp.Diff` em structs de indicadores. Melhor que `reflect.DeepEqual` |
| `github.com/stretchr/testify` | `require`/`assert` | v1.12.1. Opcional, mas reduz boilerplate |
| `golangci-lint` | Lint agregado | Configure `errcheck`, `govet`, `staticcheck`, `bodyclose` (crítico em scraper) |
| `goreleaser` v2.18.0 | Build/release multi-plataforma | Com `CGO_ENABLED=0` gera todos os alvos de uma máquina só — o payoff do SQLite puro Go |
## Installation
# TUI (ATENÇÃO ao path charm.land, não github.com/charmbracelet)
# CLI
# Coleta
# Persistência
# Planilha B3
# Infra / util
# Dev
## (1) TUI — Bubble Tea v2 e o path que quebra
### ⚠️ Achado crítico: o import path mudou
### API v2: o que mudou de verdade
- `tea.KeyMsg` → **`tea.KeyPressMsg`** (e `tea.KeyReleaseMsg`). Verificado no teste que rodou.
- `p.Start()` → `p.Run()`.
- `tea.EnterAltScreen`/`ExitAltScreen`, `tea.SetWindowTitle`, `tea.HideCursor` — **removidos**, viraram campos de `tea.View`.
- Renderizador novo ("cursed renderer") faz otimização sozinho; `tea.WithANSICompressor()` foi removido.
### Escolha do componente de tabela — a decisão mais importante da TUI
| Necessidade | `bubbles/v2/table` | `lipgloss/v2/table` | **`evertras/bubble-table`** |
|---|---|---|---|
| Ordenação por coluna | ❌ **ausente** (verificado: zero ocorrências de "sort" no fonte) | ❌ | ✅ `SortByDesc` + `ThenSortByAsc` |
| Cor **por célula** (semáforo) | ❌ só `Header`/`Cell`/`Selected` | ✅ `StyleFunc(row,col)` | ✅ `NewStyledCell` |
| Filtro / busca | ❌ | ❌ | ✅ `WithFuzzyFilter` |
| Coluna congelada | ❌ | ❌ | ✅ `WithHorizontalFreezeColumnCount` |
| Indicador de dado ausente | ❌ | ❌ | ✅ `WithMissingDataIndicator("—")` |
| Interativo (cursor, teclado) | ✅ | ❌ (renderiza string) | ✅ |
| Charm v2 | ✅ | ✅ | ✅ (v0.22.3) |
- **Tela de comparação (principal):** `evertras/bubble-table`. Ordenação numérica multi-coluna e célula colorida são exatamente os requisitos "semáforo por indicador" e "comparar lado a lado".
- **Painel de detalhe / raio-x de ativo único:** `lipgloss/v2/table` renderizado dentro de um `bubbles/v2/viewport`. É estático e rolável — mais simples e mais controlável.
- **Glossário:** `glamour` renderizando Markdown embutido dentro de um `viewport`.
- **Ajuda contextual:** `bubbles/v2/help` + `bubbles/v2/key`. Defina `KeyMap` com `key.NewBinding(key.WithKeys(...), key.WithHelp(...))` e o rodapé de atalhos se gera sozinho, sempre sincronizado.
### ⚠️ Gotcha verificado: ordenação silenciosamente errada com `StyledCell`
### Alternativas de TUI rejeitadas
| Alternativa | Versão | Veredito |
|---|---|---|
| `rivo/tview` | v0.42.0 (ago/2025) | Boa lib, widgets ricos (inclusive tabela com sort manual), API imperativa/orientada a objetos. **Rejeitada** porque o modelo MVU do Bubble Tea casa melhor com "estado derivado de dados" (recalcular semáforo/nota ao mudar pesos), e o usuário já sinalizou preferência. Ainda em `v0.x` após anos |
| `gizak/termui` | v3.1.0 — **2019** | **Morto.** 7 anos sem release. Não use |
| `jroimartin/gocui` | v0.5.0 — 2021 | Abandonado (o fork `awesome-gocui` é o vivo). Baixo nível demais |
## (2) Coleta de dados web
### Não use colly
- **Traz o que você não precisa:** fila distribuída, storage backends, `robotstxt`, detecção de charset, `xmlquery`, `mow.cli`, `sanitize` e até **`google.golang.org/appengine`** — dependência legada que vai parar no seu `go.sum`.
- **O caso de uso é minúsculo:** ~8 URLs conhecidas por execução, sem descoberta de links, sem crawl recursivo. Colly resolve *crawling*; você faz *fetching*.
- **Esconde o controle** que você mais precisa: timeout, retry, cache condicional e rate limit por host.
### ⚠️ Encoding: obrigatório, não opcional
### Estrutura da página (confirmada)
### Rate limiting respeitoso e cache
- **User-Agent identificável** — requisito explícito do PROJECT.md:
- **Rate limit conservador:** `rate.NewLimiter(rate.Every(2*time.Second), 1)` por host. Com 8 tickers isso é ~16s — irrelevante para uso pessoal sob demanda, e ordens de magnitude mais gentil que um navegador com abas.
- **Concorrência limitada:** um `errgroup` com `SetLimit(2)`, nunca 8 goroutines disparando juntas. Go torna fácil demais martelar um site pequeno.
- **Cache agressivo (a maior gentileza):** o servidor manda `cache-control: max-age=1200` (20 min). Dados fundamentalistas mudam por trimestre, não por minuto. TTL padrão de **24h** é defensável e reduz a carga em ~99%. O cache do SQLite não é otimização — é a política de boa vizinhança.
- **Retry com backoff:** só em 5xx/429/erro de rede. **Nunca** em 4xx (seletor quebrado não melhora com retry). `backoff/v5` com `MaxElapsedTime` curto e no máximo 3 tentativas.
- **`resp.Body.Close()` sempre** — habilite o linter `bodyclose`.
### APIs JSON como fonte alternativa
## (3) Persistência — SQLite
### cgo vs puro Go: decidido, e verificado
| | `modernc.org/sqlite` v1.58.0 | `mattn/go-sqlite3` v1.14.50 |
|---|---|---|
| cgo | **Não** — SQLite transpilado para Go | **Sim** — obrigatório |
| `CGO_ENABLED=0` | ✅ **Verificado** | ❌ Não compila |
| Cross-compile | ✅ **Verificado: linux/amd64, linux/arm64, darwin/arm64, darwin/amd64, windows/amd64** — tudo de uma máquina Linux, sem toolchain externo | ❌ Exige C cross-compiler por alvo (zig cc / osxcross) |
| Binário estático | ✅ **Verificado:** `ldd` → "not a dynamic executable" | Dinâmico (libc) por padrão |
| Tempo de build | Rápido, cacheável | Lento (compila ~250k linhas de C) |
| Versão SQLite | **3.53.4** (verificado em runtime) | Acompanha upstream |
| Performance | Menor que cgo em cargas pesadas | Maior |
- **"Binário único é desejável"** é constraint declarada. Com cgo, "binário único" vira "um binário por plataforma, cada um exigindo um toolchain C". Com puro Go, é `GOOS=darwin GOARCH=arm64 go build`.
- **Não há carga.** Um usuário, ~8 ativos por consulta, alguns milhares de linhas de histórico. Um `SELECT` sobre 50k linhas com índice roda em microssegundos em qualquer um dos dois. A vantagem do cgo aparece em milhões de escritas/s — cenário que este projeto nunca verá.
- **CI trivial:** `goreleaser` gera todos os alvos de um runner só.
### DSN: sintaxe do modernc é diferente
### Esquema para séries temporais
- **Indicadores diferem por classe** (FII tem P/VP e vacância; ação tem ROE e EBITDA). Tabela larga viraria um mar de `NULL` e um `ALTER TABLE` a cada indicador novo.
- **Multi-fonte com fallback** (requisito do PROJECT.md) sai de graça: `source` na PK permite guardar o mesmo indicador de duas fontes e escolher na leitura.
- **Histórico e "alerta de armadilha"** ("a dívida cresceu três anos seguidos") viram uma window function sobre `observed_at`.
- **`WITHOUT ROWID`** é a escolha certa para PK composta — armazena na própria B-tree.
- **Distinga `observed_at` (competência do dado) de `fetched_at` (quando coletamos).** Sem essa separação você não consegue exibir frescor nem detectar fonte parada.
- **`NULL` ≠ `0`.** Um DY ausente não é DY zero. Esse erro corrompe médias setoriais e semáforos.
### Migrações: goose
### Domínio financeiro
Nenhum de nós é especialista em investimentos. **Antes de escrever código que calcula,
interpreta, colore ou classifica indicador financeiro, pesquisar a convenção de mercado** e
registrar a fonte no plano ou no SUMMARY. Sem fonte, o número não sai.

A pergunta que pega o erro: *o número é calculável, mas a pergunta que ele responde faz sentido
para este ativo?* Quando não faz, mostrar ausência com motivo, nunca o número. Cuidado com o
negativo legítimo: DL/EBITDA negativo por dívida líquida negativa é notícia boa; por EBITDA
negativo é armadilha. A regra olha o sinal do denominador, não o do resultado.

Mecanismo: `derived: true` em `metrics.yaml` exige `not_applicable`, e `catalog.Load()` falha
sem ele.

### Acesso a dados
## (4) Planilha `.xlsx` da B3
| Alternativa | Veredito |
|---|---|
| `tealeg/xlsx/v3` v3.3.13 (abr/2025) | Funciona, mas menor comunidade e cadência menor. Sem vantagem aqui |
| `qax-os/excelize` v1.4.1 (2019) | **Path antigo** do mesmo projeto — o atual é `xuri/excelize/v2`. Não use |
| `encoding/csv` | Só se o usuário converter à mão. PROJECT.md confirma que a B3 entrega `.xlsx` |
- Linhas de cabeçalho/metadados antes da tabela real → **não assuma que a linha 1 é o header**; procure a linha que contém as colunas esperadas.
- Nomes de coluna em português com acento → normalize (lowercase + remova acentos) antes de casar.
- Números em formato pt-BR e/ou como texto → `GetRows` devolve `string`; **sempre parseie explicitamente**, nunca confie no tipo.
- Ticker pode vir junto do nome ("MXRF11 - MAXI RENDA FUNDO...") → extraia com regex `^([A-Z]{4}\d{1,2})`.
- **O layout muda sem aviso.** Trate a planilha como fonte não confiável: valide, e falhe com mensagem clara em vez de importar lixo.
### Parsing de números pt-BR — escreva você mesmo
## (5) Estrutura de projeto e configuração
### CLI: cobra + fang
### Layout de diretórios
- **`internal/` para tudo.** É ferramenta pessoal; nada precisa ser importável. Isso te dá liberdade total de refatorar.
- **`domain/` não importa nada de infraestrutura.** É o que permite testar semáforo e alertas sem banco nem rede.
- **`source/` esconde HTML *e* JSON atrás da mesma interface** — o requisito de fallback do PROJECT.md depende disso.
- **A TUI nunca chama a rede direto.** TUI → store → (se stale) source. Isso é o que faz "cache local + degradação explícita" funcionar, e o que mantém o `Update()` rápido.
- **Trabalho de rede/IO em `tea.Cmd`**, nunca no `Update()`. Bloquear `Update()` congela a UI inteira — o erro nº 1 em Bubble Tea.
### Config em XDG
## (6) Testes — a parte que decide se o projeto sobrevive
### Regra de ouro: nenhum teste toca a rede
### Camada 1 — Parser puro (a maioria dos testes)
### Camada 2 — Fetcher via `httptest`
### Camada 3 — Golden files (relatórios e resumos)
- Nativo: `os.WriteFile` sob uma flag `-update` + `go-cmp` para diff.
- Pronto: `sebdah/goldie/v2` (v2.8.0) ou `hexops/autogold/v2` (v2.3.1 — atualiza literais no próprio teste).
### Camada 4 — TUI com `teatest`
### Camada 5 — Store com SQLite em memória
### Determinismo: injete o relógio
## Alternatives Considered
| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| Bubble Tea v2 | `rivo/tview` | Se preferir API imperativa/widgets prontos e não gostar de MVU; tem tabela com sort embutido |
| Bubble Tea **v2** | Bubble Tea **v1** (v1.3.10) | Se precisar de lib de terceiros que só existe em v1. Custo: você começa num ramo em modo de manutenção |
| `evertras/bubble-table` | `bubbles/v2/table` + sort próprio | Se quiser zero dependências fora do Charm e topar implementar sort/filtro/cor por célula na mão |
| `net/http` + `goquery` | `gocolly/colly/v2` | Se virar crawler de verdade (descoberta de links, milhares de páginas, fila persistente) |
| `goquery` (CSS) | `antchfx/htmlquery` (XPath) | Se precisar de expressões que CSS não faz (ex.: "td cujo texto seja X, pegue o irmão") |
| `modernc.org/sqlite` | `mattn/go-sqlite3` | Só se precisar de extensões C (FTS5 custom, sqlite-vec) ou throughput de escrita massiva |
| `goose` | `golang-migrate/v4` | Se precisar de múltiplos bancos/drivers e CLI de migração robusta |
| `BurntSushi/toml` + `xdg` | `knadh/koanf/v2` | Se um dia precisar mesclar env + flags + arquivo + defaults com precedência |
| `database/sql` | `sqlc` | Se o volume de queries crescer e você quiser type-safety gerada (sem custo de runtime) |
| fixtures manuais | `dnaeon/go-vcr/v4` | Se as fontes crescerem para dezenas de endpoints e gravar na mão virar gargalo |
## What NOT to Use
| Avoid | Why | Use Instead |
|-------|-----|-------------|
| **`github.com/charmbracelet/bubbletea/v2`** | **Não compila** — o módulo declara `charm.land/bubbletea/v2`. Erro de module path | `charm.land/bubbletea/v2` |
| `gizak/termui/v3` | Último release **2019**. Abandonado | Bubble Tea v2 |
| `jroimartin/gocui` | Sem manutenção desde 2021 | Bubble Tea v2 (ou `awesome-gocui`) |
| `gocolly/colly` | Peso e dependências (incl. `google.golang.org/appengine`) para buscar ~8 URLs conhecidas; esconde o controle de rate/retry que você precisa | `net/http` + `goquery` + `x/time/rate` |
| `mattn/go-sqlite3` | Exige cgo → mata cross-compile e binário estático único | `modernc.org/sqlite` |
| `goquery` **sem** `charset.NewReader` | Fonte é ISO-8859-1 → mojibake silencioso em nomes, setores e labels | `charset.NewReader(body, contentType)` sempre |
| `qax-os/excelize` (v1) | Path legado; o projeto vive em `xuri/excelize/v2` | `github.com/xuri/excelize/v2` |
| `spf13/viper` | Dezenas de deps transitivas para um TOML local de uma ferramenta mono-usuário | `BurntSushi/toml` + `adrg/xdg` |
| GORM / ent | ORM pesado sobre 6 tabelas e queries analíticas (window functions) que ORM piora | `database/sql` (ou `sqlc`) |
| chromedp / rod (headless) | A página é HTML 4.01 server-rendered, sem JS nos dados. Traria um Chrome inteiro sem necessidade | `net/http` + `goquery` |
| `float64` para dinheiro da carteira | Erro de arredondamento acumula em custo médio e renda | `shopspring/decimal` (só em monetário) |
| `time.Now()` direto no domínio | Torna frescor e golden files não-determinísticos | Injetar `func() time.Time` |
| Guardar o `.db` em `xdg.CacheFile` | Limpador de cache apaga anos de histórico | `xdg.DataFile` |
| Zero (`0`) para dado ausente | Corrompe médias setoriais, semáforo e alertas | `NULL` + `(value, ok)` no parser |
## Stack Patterns by Variant
- `net/http` + `goquery` + **`x/net/html/charset` obrigatório**
- Guarde o HTML bruto em `raw_page` para reprocessar histórico sem rebater no site
- Testes = fixtures HTML gravadas por data
- `net/http` + `encoding/json` — **nenhuma dependência nova**
- Guarde o JSON bruto igual (mesma tabela `raw_page`)
- Atenção: brapi exige token para qualquer ticker além de PETR4/MGLU3/VALE3/ITUB4
- `source` na PK de `metric_snapshot` (já contemplado no schema)
- Precedência configurável no TOML; a leitura resolve o conflito, a escrita nunca descarta
- Registre **qual fonte** produziu cada número exibido — auditabilidade é requisito ("cálculo aberto")
- O schema append-only já constrói série histórica *incrementalmente* a partir de hoje
- Rode `goinvest sync` periodicamente (cron/systemd timer) e o histórico se acumula sozinho
- Isso não substitui 5 anos retroativos, mas garante que daqui a 1 ano você tenha 1 ano de dados reais — vale começar a coletar desde o dia 1, mesmo antes das telas prontas
## Version Compatibility
| Package A | Compatible With | Notes |
|-----------|-----------------|-------|
| `charm.land/bubbletea/v2 v2.0.9` | `charm.land/bubbles/v2 v2.2.1`, `charm.land/lipgloss/v2 v2.0.6` | Trio oficial. **Verificado compilando e rodando** |
| `github.com/evertras/bubble-table v0.22.3` | bubbletea v2.0.9 | Declara v2.0.5; MVS eleva para 2.0.9 sem problema. **Verificado** |
| `teatest/v2 (2026-09-01)` | bubbletea v2.0.9 | `go.mod` do teatest pinta `v2.0.0-rc.1`; MVS resolve para 2.0.9. **Teste verificado passando** |
| `modernc.org/sqlite v1.58.0` | `goose/v3 v3.28.0` | `goose.SetDialect("sqlite3")` + driver `"sqlite"`. **Verificado** |
| `modernc.org/sqlite v1.58.0` | `CGO_ENABLED=0` | **Verificado:** linux/{amd64,arm64}, darwin/{amd64,arm64}, windows/amd64 |
| `goquery v1.13.0` | `x/net v0.58.0` | `charset.NewReader` retorna `io.Reader` → `goquery.NewDocumentFromReader` |
| `excelize/v2 v2.11.0` | `CGO_ENABLED=0` | **Verificado** puro Go |
| Charm v2 (todos) | Go **1.25.0+** | `go.mod` do bubbles exige `go 1.25.0`. Ambiente local tem go1.26.6 ✅ |
| Bubble Tea **v1** | `charm.land/...` v2 | ❌ **Incompatíveis.** Não misture v1 e v2 no mesmo binário |
## Confidence Assessment
| Recomendação | Confiança | Base |
|---|---|---|
| Import path `charm.land/…` (v2) | **HIGH** | `go get` reproduzido: path antigo falha, novo funciona |
| Bubble Tea v2 é GA e estável | **HIGH** | Releases GitHub: v2.0.2 (mar/2026) → v2.0.9 (ago/2026), todos `prerelease: false`, só patches |
| `modernc.org/sqlite` puro Go / cross-compile | **HIGH** | Build verificado em 5 alvos + `ldd` + SQLite 3.53.4 em runtime |
| Fundamentus serve ISO-8859-1 | **HIGH** | Header HTTP + `file(1)` + mojibake reproduzido e corrigido |
| `bubbles/table` não tem sort | **HIGH** | Fonte inspecionado: zero ocorrências de "sort" |
| `evertras/bubble-table` em Charm v2 + sort/StyledCell | **HIGH** | `go.mod` inspecionado + programa compilado e executado |
| Gotcha do `StyledCell` string vs número | **HIGH** | Reproduzido errado e corrigido; `asNumber` lido no fonte |
| goose + modernc + embed | **HIGH** | Migração executada, WAL e FK confirmados via PRAGMA |
| teatest v2 × bubbletea v2.0.9 | **HIGH** | `go test` passou |
| excelize puro Go | **HIGH** | Escrita+leitura verificadas com `CGO_ENABLED=0` |
| `x/text/number` não faz parsing | **HIGH** | `doc.go` lido: "formats numbers" apenas |
| Colly ser overkill | **MEDIUM** | `go.mod` inspecionado (deps confirmadas); o julgamento de adequação é opinativo |
| brapi.dev free tier limitado a 4 tickers | **MEDIUM** | Docs oficiais; rate limits exatos não documentados na página lida |
| Layout de diretórios | **MEDIUM** | Convenção consolidada da comunidade Go, não spec normativa |
| Estrutura do HTML do Fundamentus | **MEDIUM** | Verificada hoje em 1 página (MXRF11/FII). **Ações podem ter layout diferente** — valide antes de generalizar o parser |
## Gaps / a validar na fase de implementação
- **Layout de ação vs FII vs ETF no Fundamentus.** Só o FII (MXRF11) foi inspecionado. O conjunto de labels muda por classe; grave uma fixture de cada classe antes de desenhar o parser.
- **Rate limit real das fontes.** Nenhuma publica limite. Comece conservador (0,5 req/s) e observe 429/403.
- **Layout exato do `.xlsx` da B3.** Nenhum arquivo real disponível na pesquisa. O importador precisa ser escrito contra um arquivo real do usuário — trate como descoberta da fase, não como design fechado.
- **Licença/ToS das fontes para uso pessoal.** Não avaliado aqui; vale uma leitura antes de publicar o repositório.
- **Taxonomia setorial** para comparar "pares do mesmo setor" — risco identificado no PROJECT.md, não resolvido pelo stack. É decisão de dados/modelagem.
## Sources
- **Verificação empírica local** (go1.26.6, 2026-09-02) — compilação, execução e testes de todas as recomendações centrais. **Confiança: HIGH**
- `proxy.golang.org/<módulo>/@latest` — versões correntes de todos os módulos citados. **HIGH**
- `github.com/charmbracelet/bubbletea` — `go.mod`, `UPGRADE_GUIDE_V2.md`, releases API (v2.0.2→v2.0.9). **HIGH**
- Context7 `/charmbracelet/bubbletea` — guia de migração v1→v2, assinatura de `Model`/`View`. **HIGH**
- Fonte de `charmbracelet/bubbles` (`table/table.go`), `charmbracelet/lipgloss` (`table/table.go`), `evertras/bubble-table` (`sort.go`, `options.go`, `column.go`, `data.go`). **HIGH**
- `gitlab.com/cznic/sqlite` README — driver puro Go, vtab. **HIGH**
- `pressly/goose` README — dialetos e build tags. **HIGH**
- `https://www.fundamentus.com.br/detalhes.php?papel=MXRF11` — 1 requisição, headers + HTML analisados. **HIGH**
- `https://brapi.dev/docs` — free tier e cobertura. **MEDIUM**
- `golang.org/x/text/number/doc.go` — confirmação de "formatação apenas". **HIGH**
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

### Idioma
Código em inglês (identificadores, nomes de teste, mensagens de erro ao desenvolvedor).
Comentários em pt-BR. Saída ao usuário final em pt-BR. Valores de domínio persistidos
ficam no vocabulário do mercado brasileiro mesmo com identificador em inglês:
`ClassStock AssetClass = "ACAO"`.

### Comentários
Comentar é exceção, não hábito. Só quando o porquê não é dedutível do código:
comportamento contraintuitivo de biblioteca, decisão de tipo que parece firula sem
contexto, caractere invisível, linha que um leitor futuro apagaria por parecer inútil.
Não comentar o que o código já diz. Não escrever doc de pacote em forma de ensaio.
Não citar artefato de planejamento (`Armadilha N`, `D-0X`, `RESEARCH.md`, número de
plano). Não usar travessão (`—`) em comentário.

### Go
- `internal/domain` não importa infraestrutura, verificado por `TestDomainHasNoInfraImports`.
- Todo pacote novo num teste de fronteira ganha seu import em branco em `boundaries_test.go`,
  senão o cache de teste do Go serve resultado obsoleto.
- Ausência é `nil`, nunca `0`. Parser numérico devolve `(valor, bool)`; nunca descartar
  erro de parse com `_`.

### Acesso a dados
Query nova nasce em `internal/store/queries/*.sql` e o Go sai de
`go generate ./internal/store/...` (sqlc fixado no `go.mod` via `go tool`). Nunca escrever
`database/sql` à mão para uma query nova. `internal/store` é camada fina sobre
`internal/store/gen`: converte para tipos de domínio e decide política de ausência
(`sql.ErrNoRows` vira `found=false`, nunca erro). `emit_pointers_for_null_types` mantém
`*float64` para coluna nullable: é o que impede ausência virar zero.

Não rodar `go mod tidy` enquanto houver dependência da fase sem importador: ela some do
`go.mod`.

### Commits
Assunto curto e factual; corpo só quando houver algo real a explicar. Sem citar artefato
de planejamento. Escopo `tipo(fase-plano):` é exigência do GSD.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, `.github/skills/`, or `.codex/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
