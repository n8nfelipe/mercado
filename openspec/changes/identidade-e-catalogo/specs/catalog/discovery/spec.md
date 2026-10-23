# Spec Delta

## Purpose

Define como compradores descobrem anúncios publicados por meio da vitrine, de busca textual, filtros, ordenação e da página pública do produto.

## ADDED Requirements

### Requirement: Visibilidade pública do catálogo

O sistema SHALL expor publicamente apenas anúncios com estado `published` e cuja categoria esteja ativa. Anúncios em `draft`, `pending_review`, `rejected`, `suspended` ou `archived` MUST NOT aparecer em listagens, busca, contagens ou detalhe público.

#### Scenario: Anúncio publicado aparece
- **WHEN** um comprador acessa a vitrine
- **THEN** o sistema inclui apenas anúncios `published` de categorias ativas

#### Scenario: Anúncio suspenso desaparece
- **WHEN** um anúncio publicado é suspenso
- **THEN** ele deixa de aparecer imediatamente na busca e no detalhe público

#### Scenario: Detalhe de anúncio não público
- **WHEN** um comprador solicita o detalhe de um anúncio em `draft` ou `suspended`
- **THEN** o sistema responde `404` sem revelar o conteúdo do anúncio

#### Scenario: Anúncio sem estoque
- **WHEN** um anúncio publicado possui estoque zero
- **THEN** ele permanece visível e é apresentado com indicação de indisponibilidade

### Requirement: Busca textual

O sistema SHALL buscar anúncios publicados por correspondência textual em título e descrição, sem diferenciar maiúsculas de minúsculas nem acentos. Busca vazia MUST retornar todos os anúncios publicados.

#### Scenario: Correspondência no título
- **WHEN** um comprador busca por um termo presente no título de um anúncio publicado
- **THEN** o anúncio aparece nos resultados

#### Scenario: Correspondência sem acento
- **WHEN** um comprador busca por `bicicleta` e existe anúncio com `Bicicleta` ou `bicicletá`
- **THEN** o anúncio aparece nos resultados

#### Scenario: Busca sem resultados
- **WHEN** nenhum anúncio publicado corresponde ao termo
- **THEN** o sistema retorna lista vazia com total zero e status `200`

#### Scenario: Busca vazia
- **WHEN** o comprador não informa termo de busca
- **THEN** o sistema retorna os anúncios publicados conforme filtros e ordenação aplicados

### Requirement: Filtros combináveis

O sistema SHALL aceitar filtros de categoria, preço mínimo, preço máximo, condição e frete grátis. Os filtros MUST ser combináveis com a busca e MUST ser validados, retornando `400` para valores inválidos.

#### Scenario: Filtros combinados
- **WHEN** o comprador filtra por categoria, faixa de preço e condição `used`
- **THEN** o sistema retorna apenas anúncios publicados que atendem a todos os critérios

#### Scenario: Faixa de preço inválida
- **WHEN** o preço mínimo é maior que o preço máximo ou algum preço é negativo
- **THEN** o sistema responde `400` com código `invalid_price_range`

#### Scenario: Condição inválida
- **WHEN** o comprador envia condição diferente de `new` ou `used`
- **THEN** o sistema responde `400` com erro no campo `condition`

#### Scenario: Categoria inexistente
- **WHEN** o comprador filtra por categoria inexistente
- **THEN** o sistema retorna lista vazia com total zero

### Requirement: Ordenação determinística

O sistema SHALL ordenar por relevância, menor preço, maior preço ou mais recentes, com desempate estável por identificador do anúncio. A ordenação padrão MUST ser relevância.

#### Scenario: Menor preço
- **WHEN** o comprador ordena por menor preço
- **THEN** os resultados vêm em ordem crescente de preço com desempate determinístico

#### Scenario: Mais recentes
- **WHEN** o comprador ordena por mais recentes
- **THEN** os resultados vêm em ordem decrescente de data de publicação com desempate determinístico

#### Scenario: Ordenação inválida
- **WHEN** o comprador informa uma ordenação não suportada
- **THEN** o sistema responde `400` com código `invalid_sort`

### Requirement: Paginação

O sistema SHALL paginar os resultados informando página, tamanho, total de itens e total de páginas. O tamanho máximo MUST ser 50 e o padrão MUST ser 20.

#### Scenario: Página intermediária
- **WHEN** o comprador solicita a segunda página com tamanho 10
- **THEN** o sistema retorna 10 itens correspondentes àquela página e os metadados de paginação

#### Scenario: Tamanho acima do limite
- **WHEN** o comprador solicita tamanho maior que 50
- **THEN** o sistema responde `400` com erro no campo `page_size`

#### Scenario: Página fora do intervalo
- **WHEN** o comprador solicita uma página além do total disponível
- **THEN** o sistema retorna lista vazia com total e total de páginas corretos

### Requirement: Estado da busca refletido na URL

A interface web SHALL refletir busca, filtros, ordenação e página na URL, de forma que recarregar ou compartilhar o endereço reconstrua o mesmo resultado.

#### Scenario: URL compartilhável
- **WHEN** o comprador aplica filtros e ordenação na busca
- **THEN** a URL passa a conter esses parâmetros

#### Scenario: Restauração por URL
- **WHEN** o comprador abre diretamente uma URL com parâmetros de busca
- **THEN** a página exibe o termo, filtros, ordenação e página correspondentes

### Requirement: Detalhe público do produto

O sistema SHALL exibir no detalhe público título, descrição, imagens, preço, condição, disponibilidade de estoque, frete grátis, categoria e nome público do vendedor.

#### Scenario: Detalhe completo
- **WHEN** o comprador abre um anúncio publicado com imagens e estoque
- **THEN** a página mostra todos os campos públicos e as imagens na ordem definida

#### Scenario: Dados privados omitidos
- **WHEN** o detalhe público é renderizado
- **THEN** documento do vendedor, email e outros dados privados não são incluídos na resposta
