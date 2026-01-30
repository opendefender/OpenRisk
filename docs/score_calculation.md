 Score Calculation

Ce document dcrit la formule utilise par OpenRisk pour calculer le score d'un risque, les facteurs de criticit des assets, les cas limites et des exemples de calcul.

 Formule

- Base: impact × probability
- Facteur final: moyenne des facteurs de criticit des assets associs (avg_factor)
- Score final: base × avg_factor
- Arrondi: le score est arrondi à  dcimales

En pseudo‑formule:

base = impact  probability

avg_factor = (Σ factor(asset_i)) / Nassets  (par dfaut . si pas d'assets)

final = round(base  avg_factor, )

 Facteurs de criticit (mapping)

Les facteurs appliqus aux assets sont les suivants (voir backend/internal/services/score_service.go):

- Low      → .
- Medium   → .
- High     → .
- Critical → .

Si un asset possde une criticit inconnue, le facteur par dfaut . est utilis.

 Cas limites

- Aucun asset associ: avg_factor = . → final = base
- Asset(s) avec criticit non reconnue: ces assets comptent pour . dans la moyenne
- Impact/probability hors plage attendue (-): la validation API empêche normalement ces valeurs; si prsente, la fonction calcule nanmoins avec les valeurs reçues

 Exemple  — Pas d'assets

- impact = , probability = 
- base = 
- avg_factor = .
- final = .

 Exemple  — Plusieurs assets

- impact = , probability =  → base = 
- assets: [Low (.), High (.)] → avg_factor = (. + .) /  = .
- final =  × . = .

 Emplacement du code

- Implmentation: backend/internal/services/score_service.go (fonction ComputeRiskScore)
- Tests unitaires: backend/internal/services/score_service_test.go
- Appels depuis handlers: backend/internal/handlers/risk_handler.go (CreateRisk et UpdateRisk)

 Recommandations

- Documenter toute modification des facteurs dans ce fichier et ajouter un test unitaire correspondant.
- Si vous souhaitez une autre formule (p. ex. poids non linaires ou minimum/maximum), proposer un ticket/PR et ajouter une migration de tests.

---

Fichier gnr automatiquement par l'outil de documentation interne.