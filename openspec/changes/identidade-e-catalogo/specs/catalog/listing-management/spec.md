# Spec Delta

## Purpose

Define como administradores mantêm categorias e como vendedores criam, ilustram, estocam, submetem e mantêm anúncios, incluindo as regras de moderação e de visibilidade pública.

## ADDED Requirements

### Requirement: Gestão de categorias

O sistema SHALL permitir que administradores criem, editem, ativem e desativem categorias com nome e slug únicos. Uma categoria MUST poder ter uma categoria pai e MUST NOT ser removida enquanto possuir anúncios associados.

#### Scenario: Categoria criada
- **WHEN** um administrador envia nome e slug inéditos
- **THEN** o sistema cria a categoria ativa e responde com os dados criados

#### Scenario: Slug duplicado
- **WHEN** um administrador tenta criar ou renomear uma categoria para um slug já usado
- **THEN** o sistema responde `409` com código `category_slug_taken`

#### Scenario: Remoção bloqueada
- **WHEN** um administrador tenta remover uma categoria que possui anúncios
- **THEN** o sistema responde `409` com código `category_in_use`

#### Scenario: Categoria inativa
- **WHEN** uma categoria é desativada
- **THEN** ela deixa de aceitar novos anúncios e deixa de aparecer na navegação pública, mas os anúncios existentes permanecem consultáveis

### Requirement: Criação de anúncio

O sistema SHALL permitir que um vendedor com perfil completo crie um anúncio em estado `draft` com título, descrição, categoria, preço em centavos, moeda, condição, frete grátis e estoque inicial.

#### Scenario: Anúncio criado
- **WHEN** um vendedor envia todos os campos obrigatórios válidos
- **THEN** o sistema cria o anúncio em `draft`, associado ao vendedor, e responde `201`

#### Scenario: Usuário sem papel de vendedor
- **WHEN** um comprador sem onboarding de vendedor tenta criar um anúncio
- **THEN** o sistema responde `403` com código `seller_profile_required`

#### Scenario: Categoria inativa ou inexistente
- **WHEN** um vendedor envia uma categoria inativa ou inexistente
- **THEN** o sistema responde `400` com erro no campo `category_id`

### Requirement: Validação dos dados do anúncio

O anúncio MUST ter título entre 10 e 120 caracteres, descrição entre 20 e 5000 caracteres, preço inteiro positivo em centavos, moeda `BRL`, condição `new` ou `used` e estoque entre 0 e 1.000.000.

#### Scenario: Preço inválido
- **WHEN** um vendedor envia preço zero, negativo ou fracionário
- **THEN** o sistema responde `400` com erro no campo `price_cents`

#### Scenario: Título fora do limite
- **WHEN** um vendedor envia título com menos de 10 ou mais de 120 caracteres
- **THEN** o sistema responde `400` com erro no campo `title`

#### Scenario: Condição inválida
- **WHEN** um vendedor envia condição diferente de `new` ou `used`
- **THEN** o sistema responde `400` com erro no campo `condition`

### Requirement: Ciclo de vida do anúncio

O sistema SHALL controlar os estados `draft`, `pending_review`, `published`, `rejected`, `suspended` e `archived`, permitindo apenas as transições: `draft` para `pending_review`, `pending_review` para `published` ou `rejected`, `rejected` para `pending_review`, `published` para `suspended` ou `archived`, `suspended` para `published` ou `archived` e `draft` para `archived`.

#### Scenario: Submissão para revisão
- **WHEN** o vendedor proprietário submete um anúncio em `draft` ou `rejected` para revisão
- **THEN** o anúncio passa a `pending_review` e recebe data de submissão

#### Scenario: Transição inválida
- **WHEN** o vendedor tenta submeter um anúncio já `published` ou `suspended` para revisão
- **THEN** o sistema responde `409` com código `invalid_listing_state`

#### Scenario: Anúncio arquivado
- **WHEN** o vendedor arquiva um anúncio em `draft`, `published` ou `suspended`
- **THEN** o anúncio passa a `archived` e deixa de ser editável

### Requirement: Edição e propriedade do anúncio

Somente o vendedor proprietário SHALL editar o conteúdo do anúncio em `draft` ou `rejected`. Em anúncios `published` ou `suspended`, apenas preço e estoque MUST ser editáveis. Alterações de conteúdo em anúncio publicado MUST exigir nova submissão para revisão.

#### Scenario: Edição de rascunho
- **WHEN** o proprietário edita título, descrição, categoria, preço ou estoque de um anúncio em `draft`
- **THEN** o sistema persiste as alterações e mantém o estado `draft`

#### Scenario: Edição de conteúdo publicado
- **WHEN** o proprietário altera título ou descrição de um anúncio `published`
- **THEN** o sistema responde `409` com código `listing_requires_review`

#### Scenario: Edição por terceiro
- **WHEN** outro vendedor tenta editar o anúncio
- **THEN** o sistema responde `403` e não altera dados

### Requirement: Upload de imagens do anúncio

O sistema SHALL aceitar imagens JPEG, PNG ou WebP de até 5 MiB por arquivo, com no máximo 8 imagens por anúncio. O tipo MUST ser validado pelo conteúdo do arquivo, não pela extensão. Apenas o proprietário pode enviar ou remover imagens.

#### Scenario: Upload válido
- **WHEN** o proprietário envia uma imagem válida para seu anúncio
- **THEN** o sistema armazena o objeto, registra a imagem e retorna sua URL

#### Scenario: Arquivo grande
- **WHEN** o proprietário envia imagem acima de 5 MiB
- **THEN** o sistema responde `413` com código `image_too_large`

#### Scenario: Tipo não suportado
- **WHEN** o proprietário envia arquivo cujo conteúdo não é JPEG, PNG nem WebP
- **THEN** o sistema responde `415` com código `unsupported_image_type`

#### Scenario: Limite de imagens
- **WHEN** o proprietário tenta adicionar uma nona imagem
- **THEN** o sistema responde `409` com código `image_limit_reached`

### Requirement: Estoque do anúncio

O sistema SHALL manter a quantidade disponível do anúncio e MUST registrar cada ajuste com autor, motivo e data. A quantidade MUST NOT ficar negativa.

#### Scenario: Ajuste de estoque
- **WHEN** o proprietário ajusta o estoque informando quantidade e motivo
- **THEN** o sistema atualiza a quantidade e registra o ajuste

#### Scenario: Estoque negativo
- **WHEN** um ajuste resultaria em quantidade negativa
- **THEN** o sistema responde `409` com código `insufficient_stock` e mantém a quantidade anterior

#### Scenario: Estoque zerado
- **WHEN** o estoque de um anúncio publicado chega a zero
- **THEN** o anúncio continua publicado, mas é apresentado como indisponível e não pode ser comprado

### Requirement: Moderação de anúncios

O sistema SHALL permitir que administradores publiquem, rejeitem ou suspendam anúncios. Rejeição e suspensão MUST exigir motivo. Toda decisão MUST registrar administrador, data e motivo.

#### Scenario: Aprovação
- **WHEN** um administrador publica um anúncio em `pending_review`
- **THEN** o anúncio passa a `published`, recebe data de publicação e fica visível na busca pública

#### Scenario: Rejeição
- **WHEN** um administrador rejeita um anúncio em `pending_review` com motivo
- **THEN** o anúncio passa a `rejected` e o motivo fica disponível ao vendedor

#### Scenario: Suspensão
- **WHEN** um administrador suspende um anúncio publicado com motivo
- **THEN** o anúncio passa a `suspended` e deixa imediatamente de aparecer na busca pública

#### Scenario: Fila de moderação
- **WHEN** um administrador consulta anúncios pendentes
- **THEN** o sistema retorna apenas anúncios em `pending_review`, ordenados do mais antigo para o mais recente
