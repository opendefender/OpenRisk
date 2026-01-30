 OpenRisk - Cas d'Usage Rels

Ce document prsente  cas d'usage concrets où OpenRisk cre de la valeur immdiate.

---

  Cas : Startup SaaS - Mesurer & Prioriser les Risques Prod

 Le Problme
TechStart.io est une startup SaaS avec  employs et  clients. Leur infrastructure grandit mais leur processus de gestion des risques est artisanal:
- Risques documents dans Google Sheets
- Pas de scoring centralis
- Les alertes scurit s'accumulent sans priorisation
- CISO travaille h/semaine à tracker manuellement

 Solution avec OpenRisk

 ⃣ Configuration Initiale ( min)
bash
 Dmarrer OpenRisk
docker compose up -d

 Accder à l'interface
 → http://localhost:
 Email: admin@openrisk.local | Password: admin


 ⃣ Crer les Catgories de Risques
Depuis l'interface:
- Infrastructure (serveurs, bases de donnes, rseaux)
- Application (bugs, vulnrabilits logicielles)
- Data (fuites, conformit RGPD)
- Oprations (incidents, RTO/RPO)

 ⃣ Évaluer les Risques Existants
Exemple: Vulnrabilit dans Node.js v


Titre: Vulnrabilit Node.js  - Injection HTTP
Description: Un attaquant peut envoyer des headers malveillants
Framework: OWASP Top  - Injection
Criticit: Haute (Availability)
Probabilit: Moyenne (besoin d'exploitation)

Score Automatique: ./ (Haute Priorit)


 ⃣ Crer le Plan d'Attnuation

Mitigation: Upgrade Node.js  →  LTS
Status: En Cours
Responsable: DevOps Lead
Deadline:  janvier 

Sub-actions (Checklist):
 Tester sur environnement staging
 Valider les dpendances
 Dployer en prod
 Monitoring h aprs dploiement


 ⃣ Dashboard Temps Rel
Le CISO voit en un coup d'œil:
-  risques Hauts → Demandent action immdiate
-  risques Moyens → À planifier
-  risques Bas → À monitorer
- Graphique de tendance → Montre  risques rsolus ce mois-ci

  Impact Rel
| Avant | Aprs |
|-------|-------|
| h/semaine de gestion manuelle | h/semaine de suivi |
| Pas de visibilit pour l'quipe exec | Dashboard en temps rel |
| Risques oublis | % tracs |
| Rapports mensuels = urgence | Rapports gnrs en  clics |

Rsultat: Le CISO peut se concentrer sur la stratgie au lieu de l'administratif.

---

  Cas : PME - Centraliser les Alertes Scurit

 Le Problme
SecureLogistics.fr est une PME de  employs avec une infrastructure hybride:
- Serveurs on-premise + AWS
- Elastic Stack pour les logs
- Splunk pour la scurit
- Les alertes arrivent partout: mail, Slack, tickets Jira
- Impossible de tracker "qui doit faire quoi"

 Solution avec OpenRisk

 ⃣ Importer les Donnes Existantes
OpenRisk peut se connecter à vos outils existants:

bash
 Configuration dans l'interface (Settings → Integrations)

 Option : Splunk Integration
API_SPLUNK_URL=https://splunk.securelog.fr:
API_SPLUNK_TOKEN=xxxxx
IMPORT_ALERTS=true

 Option : Elastic Integration  
ELASTICSEARCH_URL=https://elastic.securelog.fr:
IMPORT_ALERTS=true

 Option : Manuel (importer un CSV)
 Uploadez votre fichier dans OpenRisk


 ⃣ Exemple: Alerte Splunk "Connexion SSH Brute-Force"

L'alerte arrive:

[CRITICAL]  tentatives SSH choues sur srv-prod-
Source: ...
Temps: -- ::


Dans OpenRisk:
- Crer un Risque: "Attaque par force brute sur SSH"
- Scorer automatiquement: ./ (Critre: tentatives rptes + prod)
- Assigner à: Responsable Infrastructure
- Lier à Mitigation: "Implmenter failban"
- Sub-actions:
  
   Bloquer l'IP immdiatement
   Vrifier si accs granted
   Implmenter rate limiting
   Ajouter FA obligatoire
  

 ⃣ Tableau de Bord Centralis
Un seul endroit pour voir:
-  Critiques actifs: 
-  Hauts: 
-  Moyens: 
-  Bas: 
- Graphique: Tendance des  derniers jours

 ⃣ Intgration Team

Slack Integration:
- Notification quand nouveau risque Critique
- Daily digest des  risques à traiter
- Rapport hebdomadaire


  Impact Rel
| Avant | Aprs |
|-------|-------|
| Alertes disperses = beaucoup oublies | % centralis |
| -h de temps pour chercher "où est l'alerte" | s pour retrouver l'info |
| Pas d'ordre de priorit | Score automatique qui trie |
| Responsabilits floues | Chaque risque a un proprio |

Rsultat: Les alertes deviennent des actions traces, plus du bruit.

---

  Cas : RSSI - Rapports Trimestriels Automatiss

 Le Problme
MegatechCorp.com est une grande entreprise avec  employs. Le RSSI doit:
- Produire un rapport de conformit chaque trimestre
- Montrer les risques identifis
- Prouver que les mitigations avancent
- Remettre à la direction + auditeurs externes
- Actuellement:  jours de travail par rapport

 Solution avec OpenRisk

 ⃣ Configuration Annuelle ( heure)

bash
 Dans Settings → Organization
Compliance_Framework: ISO 
Report_Frequency: Trimestrel
Auto_Export_Format: PDF + Excel
Recipients: 
  - direction@megatech.fr
  - audit@megatech.fr
  - ciso@megatech.fr


 ⃣ Exemple de Rapport Q 

OpenRisk gnre automatiquement:


 RAPPORT TRIMESTRIEL - GESTION DES RISQUES
Priode: Oct - Dc 
Gnr le:  Dcembre 

. RÉSUMÉ EXÉCUTIF
     risques identifis
     risques rsolus ce trimestre (-%)
     mitigations en cours (deadline: Q )
      risques Critiques remonts à la Direction

. TENDANCES
   [Graphique] Évolution du nombre de risques
   - Trend: ↓ -% vs Q (Positif!)
   - Rsolutions:  risques
   - Nouveaux:  risques

. DÉTAIL PAR DOMAINE
   
   Infrastructure:  risques
    Critiques:  (Vieux serveur Windows XP)
    Hauts: 
    Moyens: 

   Application:  risques
    Critiques:  (Dpendances outdated)
    Hauts: 
    Moyens: 

   Data & Compliance:  risques
    Critiques: 
    Hauts: 
    Moyens: 

. MITIGATIONS EN COURS
   
    Upgrade Node.js (% complete)
       Deadline:  Jan 
   
    Implmenter MFA (% complete)
       Deadline:  Feb 
   
    Audit scurit externe (% complete)
       Deadline:  Mar 

. CONFORMITÉ
   ISO :  % couvert (vs % Q)
   RGPD:  % couvert
   SOC:  % en cours

. RECOMMANDATIONS
   - Acclrer l'upgrade Node.js (Critique)
   - Implmenter MFA immdiatement (Scurit)
   - Refondre l'architecture legacy (Moyen terme)

---
Sign numriquement par OpenRisk v..


 ⃣ Exporter le Rapport

Depuis OpenRisk:
bash
 Interface: Reports → Download Trimestral Report
 Formats disponibles:
 - PDF (prêt à imprimer)
 - Excel (pour analyse)
 - JSON (pour BI tools)


 ⃣ Temps Ncessaire

Avant:  jours (collecte manuelle + mise en forme)

Jour : Envoyer des mails aux quipes
Jour -: Collecter les rponses
Jour : Formatter en PowerPoint
Jour : Validation + corrections


Avec OpenRisk:  minutes

. Click "Generate Quarterly Report"
. Tlcharger PDF
. Envoyer aux stakeholders


  Impact Rel
| Avant | Aprs |
|-------|-------|
|  jours/mois de prparation |  min/trimestre |
| Donnes potentiellement outdated | Donnes en temps rel |
| Impossible de tracker l'volution | Graphiques de tendance |
| Format varie chaque fois | Format cohrent & professionnel |

Rsultat: Le RSSI peut justifier son budget auprs de la direction avec donnes prcises.

---

  Synthse: Pourquoi OpenRisk?

 Pour les Startups
 Automatiser = moins de temps manuel  
 Prioriser = focuser sur ce qui compte  
 Écheller = passer de  à  risques facilement

 Pour les PME
 Centraliser = une source de vrit  
 Intgrer = connecter outils existants  
 Rapporter = prouver la scurit

 Pour les Entreprises
 Automatiser = conomiser + jours/an par RSSI  
 Auditer = rapports conformit en  min  
 Gouverner = visibilit complte pour la direction

---

  Prêt à essayer?

[→ Dmarrer en  minutes](QUICK_ONBOARDING.md)

Des questions? Consultez [API_REFERENCE.md](API_REFERENCE.md) ou ouvrez une [discussion](https://github.com/alex-dembele/OpenRisk/discussions).
