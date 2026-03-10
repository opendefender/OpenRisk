# Score Engine - Formule de Calcul des Risques

## Vue d'ensemble

Le Score Engine d'OpenRisk est le moteur central de calcul qui évalue automatiquement les risques en utilisant une formule mathématique basée sur la probabilité, l'impact et la criticité des assets.

## Formule Principale

### Calcul de Base

```
Risk Score = Probability × Impact × Average(Asset Criticality Factors)
```

### Décomposition

#### 1. Score de Base
```
Base Score = Probability × Impact
```

- **Probability** (1-5) : Likelihood d'occurrence du risque
- **Impact** (1-5) : Severity si le risque se matérialise

**Exemple** : Probability=3, Impact=4 → Base Score = 12

#### 2. Facteur de Criticité des Assets
```
Asset Criticality Factor = Average of all asset criticality multipliers
```

Les facteurs de criticité sont définis comme suit :
- **Low (Faible)** : 0.8x - Assets peu critiques
- **Medium (Moyen)** : 1.0x - Assets standards
- **High (Élevé)** : 1.25x - Assets importants
- **Critical (Critique)** : 1.5x - Assets vitaux pour l'organisation

**Exemple** : Assets avec criticités [Low, High]
```
Average Factor = (0.8 + 1.25) / 2 = 1.025
```

#### 3. Score Final
```
Final Score = Base Score × Average(Asset Criticality Factors)
```

**Exemple complet** :
- Base Score = 3 × 4 = 12
- Assets Criticality = [Low, High] → Average = 1.025
- Final Score = 12 × 1.025 = **12.3**

#### 4. Cas sans Assets
Si aucun asset n'est associé au risque :
```
Final Score = Base Score × 1.0
```

## Classification des Niveaux de Risque

Basé sur le Score Final, le risque est classifié en 4 niveaux :

| Niveau | Intervalle de Score | Seuil |
|--------|---------------------|-------|
| **LOW** (Faible) | 0 - 5 | < 5 |
| **MEDIUM** (Moyen) | 6 - 12 | 5 - 12 |
| **HIGH** (Élevé) | 13 - 19 | 13 - 19 |
| **CRITICAL** (Critique) | 20+ | ≥ 20 |

### Matrice de Risque Standard

```
             Probability
        1    2    3    4    5
    5  [ 5] [10] [15] [20] [25]  CRITICAL
    4  [ 4] [ 8] [12] [16] [20]  HIGH
Impact
    3  [ 3] [ 6] [ 9] [12] [15]  MEDIUM
    2  [ 2] [ 4] [ 6] [ 8] [10]  LOW
    1  [ 1] [ 2] [ 3] [ 4] [ 5]  LOW
```

## Fonctionnalités Avancées

### 1. Configurations Personnalisables

#### Pondérations Personnalisées
Les administrateurs peuvent ajuster les facteurs de pondération pour adapter le calcul à leur contexte :

```json
{
  "id": "custom-config",
  "name": "Configuration Personnalisée",
  "base_formula": "impact*probability",
  "weighting_factors": {
    "impact": 1.2,      // Augmente le poids de l'impact
    "probability": 0.9, // Réduit le poids de la probabilité
    "criticality": 1.0,
    "trend": 0.1        // Ajustement basé sur la tendance
  }
}
```

#### Matrices de Risque Personnalisées
```json
{
  "risk_matrix_thresholds": {
    "low": 4,
    "medium": 10,
    "high": 18,
    "critical": 24
  }
}
```

### 2. Ajustement Dynamique basé sur les Tendances

Les scores peuvent être ajustés dynamiquement basé sur la tendance (evolution du risque) :

```
Adjusted Score = Base Score × (1 + trend_weight × trend_factor)
```

- **trend_factor** positif : risque en augmentation
- **trend_factor** négatif : risque en diminution
- **trend_weight** : configuré dans les weighting_factors

**Exemple** :
- Base Score = 10
- Trend Weight = 0.2
- Trend Factor = 0.1 (tendance positive légère)
- Adjusted Score = 10 × (1 + 0.2 × 0.1) = 10.2

### 3. Calcul Automatique

#### À la Création
Lors de la création d'un risque, le score est calculé automatiquement :
```go
POST /api/v1/risks
{
  "title": "Data Breach",
  "impact": 4,
  "probability": 3,
  "asset_ids": ["asset-1", "asset-2"]
}
// Score calculé automatiquement lors de la sauvegarde
```

#### Lors des Modifications
À chaque modification (Impact, Probability, Assets), le score est recalculé :
```go
PATCH /api/v1/risks/:id
{
  "impact": 5,
  "probability": 4
}
// Score recalculé automatiquement
```

## Endpoints API

### Calcul de Score

```http
POST /api/v1/score-engine/compute
Content-Type: application/json

{
  "impact": 4,
  "probability": 3,
  "asset_ids": ["asset-1", "asset-2"],
  "config_id": "default",
  "apply_trend": true,
  "trend_factor": 0.1
}

Response:
{
  "base_score": 12.0,
  "final_score": 12.3,
  "risk_level": "HIGH",
  "impact": 4,
  "probability": 3,
  "asset_count": 2
}
```

### Classification

```http
POST /api/v1/score-engine/classify
Content-Type: application/json

{
  "score": 15.5,
  "config_id": "default"
}

Response:
{
  "score": 15.5,
  "risk_level": "HIGH",
  "config_id": "default",
  "matrix": {
    "low": 5,
    "medium": 12,
    "high": 19,
    "critical": 20
  }
}
```

### Récupération de la Matrice

```http
GET /api/v1/score-engine/matrix?config_id=default

Response:
{
  "matrix": {
    "low": 5,
    "medium": 12,
    "high": 19,
    "critical": 20
  },
  "config_id": "default",
  "formula": "impact*probability",
  "weighting": {...},
  "criticality": {...}
}
```

### Métriques de Scoring

```http
GET /api/v1/score-engine/metrics

Response:
{
  "avg_score": 12.5,
  "max_score": 25.0,
  "risk_stats": [
    {"level": "critical", "count": 5},
    {"level": "high", "count": 12},
    {"level": "medium", "count": 23},
    {"level": "low", "count": 45}
  ]
}
```

## Configuration Management

### Créer une Configuration

```http
POST /api/v1/score-engine/configs
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "name": "Configuration pour Infrastructure Critique",
  "description": "Configuration adaptée pour les assets critiques",
  "base_formula": "impact*probability",
  "weighting_factors": {
    "impact": 1.3,
    "probability": 1.0,
    "criticality": 1.2,
    "trend": 0.15
  },
  "risk_matrix_thresholds": {
    "low": 4,
    "medium": 10,
    "high": 18,
    "critical": 24
  },
  "asset_criticality_mult": {
    "low": 0.7,
    "medium": 1.0,
    "high": 1.4,
    "critical": 1.8
  }
}
```

### Récupérer une Configuration

```http
GET /api/v1/score-engine/configs/{id}

Response:
{
  "id": "custom-config",
  "name": "Configuration Personnalisée",
  "base_formula": "impact*probability",
  "weighting_factors": {...},
  "risk_matrix_thresholds": {...},
  "asset_criticality_mult": {...}
}
```

### Mettre à Jour une Configuration

```http
PUT /api/v1/score-engine/configs/{id}
Authorization: Bearer {admin_token}
Content-Type: application/json

{
  "weighting_factors": {
    "impact": 1.5
  }
}
```

## Tests Unitaires

Le Score Engine est couvert par une suite de tests unitaires complète :

- ✅ Configuration par défaut
- ✅ Calcul sans assets
- ✅ Calcul avec assets
- ✅ Pondérations personnalisées
- ✅ Classification des niveaux de risque
- ✅ Ajustement basé sur les tendances
- ✅ Validation des configurations
- ✅ Opérations CRUD des configurations
- ✅ Gestion de la matrice de risque

Exécution des tests :
```bash
cd backend
go test ./internal/services -v -run TestScore
go test ./internal/services -v -run TestComputeRisk
```

## Évolutions Futures (Roadmap)

### Phase 1 : Pondération Avancée
- [ ] Pondérations par framework de conformité
- [ ] Facteurs temporels (urgence, SLA)
- [ ] Pondérations par département/équipe

### Phase 2 : Analyse Dynamique
- [ ] Scores basés sur les tendances historiques
- [ ] Modèle prédictif de risque
- [ ] Analyse d'impact à grande échelle

### Phase 3 : Intelligence Artificielle
- [ ] Machine Learning pour ajustements automatiques
- [ ] Détection d'anomalies de risque
- [ ] Recommandations d'ajustement de score

## Notes Importantes

1. **Arrondi** : Les scores sont arrondis à 2 décimales
2. **Assets vides** : Un risque sans asset associé utilise un facteur de 1.0
3. **Recalcul automatique** : Le score est recalculé à chaque modification de Impact, Probability ou Assets
4. **Immutabilité** : La configuration par défaut ne peut pas être supprimée
5. **Permissions** : Seuls les administrateurs peuvent créer/modifier les configurations

## Exemple Complet

### Scénario
Vous créez un risque "Data Breach" avec :
- Title: "Potential Data Breach in Production DB"
- Impact: 5 (Très élevé - données clients)
- Probability: 3 (Moyen - pas de vulnérabilité connue)
- Assets: [Production_DB (Critical), Backup_Server (High)]

### Calcul
1. Base Score = 5 × 3 = 15
2. Asset Factors = [1.5, 1.25] → Average = 1.375
3. Final Score = 15 × 1.375 = **20.625**
4. Classification = **CRITICAL** (≥ 20)

### Résultat
Le risque est créé avec :
- Score: 20.625
- Risk Level: CRITICAL
- Nécessite une attention immédiate et un plan de traitement
