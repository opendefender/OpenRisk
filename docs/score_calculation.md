 Score Calculation

Ce document d√crit la formule utilis√e par OpenRisk pour calculer le score d'un risque, les facteurs de criticit√ des assets, les cas limites et des exemples de calcul.

 Formule

- Base: impact √ó probability
- Facteur final: moyenne des facteurs de criticit√ des assets associ√s (avg_factor)
- Score final: base √ó avg_factor
- Arrondi: le score est arrondi √†  d√cimales

En pseudo‚Äëformule:

base = impact  probability

avg_factor = (Œ£ factor(asset_i)) / Nassets  (par d√faut . si pas d'assets)

final = round(base  avg_factor, )

 Facteurs de criticit√ (mapping)

Les facteurs appliqu√s aux assets sont les suivants (voir backend/internal/services/score_service.go):

- Low      ‚Üí .
- Medium   ‚Üí .
- High     ‚Üí .
- Critical ‚Üí .

Si un asset poss√de une criticit√ inconnue, le facteur par d√faut . est utilis√.

 Cas limites

- Aucun asset associ√: avg_factor = . ‚Üí final = base
- Asset(s) avec criticit√ non reconnue: ces assets comptent pour . dans la moyenne
- Impact/probability hors plage attendue (-): la validation API emp√™che normalement ces valeurs; si pr√sente, la fonction calcule n√anmoins avec les valeurs re√ßues

 Exemple  ‚Äî Pas d'assets

- impact = , probability = 
- base = 
- avg_factor = .
- final = .

 Exemple  ‚Äî Plusieurs assets

- impact = , probability =  ‚Üí base = 
- assets: [Low (.), High (.)] ‚Üí avg_factor = (. + .) /  = .
- final =  √ó . = .

 Emplacement du code

- Impl√mentation: backend/internal/services/score_service.go (fonction ComputeRiskScore)
- Tests unitaires: backend/internal/services/score_service_test.go
- Appels depuis handlers: backend/internal/handlers/risk_handler.go (CreateRisk et UpdateRisk)

 Recommandations

- Documenter toute modification des facteurs dans ce fichier et ajouter un test unitaire correspondant.
- Si vous souhaitez une autre formule (p. ex. poids non lin√aires ou minimum/maximum), proposer un ticket/PR et ajouter une migration de tests.

---

Fichier g√n√r√ automatiquement par l'outil de documentation interne.