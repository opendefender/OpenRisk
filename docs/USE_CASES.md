 OpenRisk - Cas d'Usage R√els

Ce document pr√sente  cas d'usage concrets o√π OpenRisk cr√e de la valeur imm√diate.

---

  Cas : Startup SaaS - Mesurer & Prioriser les Risques Prod

 Le Probl√me
TechStart.io est une startup SaaS avec  employ√s et  clients. Leur infrastructure grandit mais leur processus de gestion des risques est artisanal:
- Risques document√s dans Google Sheets
- Pas de scoring centralis√
- Les alertes s√curit√ s'accumulent sans priorisation
- CISO travaille h/semaine √† tracker manuellement

 Solution avec OpenRisk

 ‚É£ Configuration Initiale ( min)
bash
 D√marrer OpenRisk
docker compose up -d

 Acc√der √† l'interface
 ‚Üí http://localhost:
 Email: admin@openrisk.local | Password: admin


 ‚É£ Cr√er les Cat√gories de Risques
Depuis l'interface:
- Infrastructure (serveurs, bases de donn√es, r√seaux)
- Application (bugs, vuln√rabilit√s logicielles)
- Data (fuites, conformit√ RGPD)
- Op√rations (incidents, RTO/RPO)

 ‚É£ √âvaluer les Risques Existants
Exemple: Vuln√rabilit√ dans Node.js v


Titre: Vuln√rabilit√ Node.js  - Injection HTTP
Description: Un attaquant peut envoyer des headers malveillants
Framework: OWASP Top  - Injection
Criticit√: Haute (Availability)
Probabilit√: Moyenne (besoin d'exploitation)

Score Automatique: ./ (Haute Priorit√)


 ‚É£ Cr√er le Plan d'Att√nuation

Mitigation: Upgrade Node.js  ‚Üí  LTS
Status: En Cours
Responsable: DevOps Lead
Deadline:  janvier 

Sub-actions (Checklist):
‚òë Tester sur environnement staging
‚òë Valider les d√pendances
‚òê D√ployer en prod
‚òê Monitoring h apr√s d√ploiement


 ‚É£ Dashboard Temps R√el
Le CISO voit en un coup d'≈ìil:
-  risques Hauts ‚Üí Demandent action imm√diate
-  risques Moyens ‚Üí √Ä planifier
-  risques Bas ‚Üí √Ä monitorer
- Graphique de tendance ‚Üí Montre  risques r√solus ce mois-ci

  Impact R√el
| Avant | Apr√s |
|-------|-------|
| h/semaine de gestion manuelle | h/semaine de suivi |
| Pas de visibilit√ pour l'√quipe exec | Dashboard en temps r√el |
| Risques oubli√s | % trac√s |
| Rapports mensuels = urgence | Rapports g√n√r√s en  clics |

R√sultat: Le CISO peut se concentrer sur la strat√gie au lieu de l'administratif.

---

  Cas : PME - Centraliser les Alertes S√curit√

 Le Probl√me
SecureLogistics.fr est une PME de  employ√s avec une infrastructure hybride:
- Serveurs on-premise + AWS
- Elastic Stack pour les logs
- Splunk pour la s√curit√
- Les alertes arrivent partout: mail, Slack, tickets Jira
- Impossible de tracker "qui doit faire quoi"

 Solution avec OpenRisk

 ‚É£ Importer les Donn√es Existantes
OpenRisk peut se connecter √† vos outils existants:

bash
 Configuration dans l'interface (Settings ‚Üí Integrations)

 Option : Splunk Integration
API_SPLUNK_URL=https://splunk.securelog.fr:
API_SPLUNK_TOKEN=xxxxx
IMPORT_ALERTS=true

 Option : Elastic Integration  
ELASTICSEARCH_URL=https://elastic.securelog.fr:
IMPORT_ALERTS=true

 Option : Manuel (importer un CSV)
 Uploadez votre fichier dans OpenRisk


 ‚É£ Exemple: Alerte Splunk "Connexion SSH Brute-Force"

L'alerte arrive:

[CRITICAL]  tentatives SSH √chou√es sur srv-prod-
Source: ...
Temps: -- ::


Dans OpenRisk:
- Cr√er un Risque: "Attaque par force brute sur SSH"
- Scorer automatiquement: ./ (Crit√re: tentatives r√p√t√es + prod)
- Assigner √†: Responsable Infrastructure
- Lier √† Mitigation: "Impl√menter failban"
- Sub-actions:
  
  ‚òë Bloquer l'IP imm√diatement
  ‚òê V√rifier si acc√s granted
  ‚òê Impl√menter rate limiting
  ‚òê Ajouter FA obligatoire
  

 ‚É£ Tableau de Bord Centralis√
Un seul endroit pour voir:
- üî Critiques actifs: 
- üü† Hauts: 
- üü° Moyens: 
- üü¢ Bas: 
- Graphique: Tendance des  derniers jours

 ‚É£ Int√gration Team

Slack Integration:
- Notification quand nouveau risque Critique
- Daily digest des  risques √† traiter
- Rapport hebdomadaire


  Impact R√el
| Avant | Apr√s |
|-------|-------|
| Alertes dispers√es = beaucoup oubli√es | % centralis√ |
| -h de temps pour chercher "o√π est l'alerte" | s pour retrouver l'info |
| Pas d'ordre de priorit√ | Score automatique qui trie |
| Responsabilit√s floues | Chaque risque a un proprio |

R√sultat: Les alertes deviennent des actions trac√es, plus du bruit.

---

  Cas : RSSI - Rapports Trimestriels Automatis√s

 Le Probl√me
MegatechCorp.com est une grande entreprise avec  employ√s. Le RSSI doit:
- Produire un rapport de conformit√ chaque trimestre
- Montrer les risques identifi√s
- Prouver que les mitigations avancent
- Remettre √† la direction + auditeurs externes
- Actuellement:  jours de travail par rapport

 Solution avec OpenRisk

 ‚É£ Configuration Annuelle ( heure)

bash
 Dans Settings ‚Üí Organization
Compliance_Framework: ISO 
Report_Frequency: Trimestrel
Auto_Export_Format: PDF + Excel
Recipients: 
  - direction@megatech.fr
  - audit@megatech.fr
  - ciso@megatech.fr


 ‚É£ Exemple de Rapport Q 

OpenRisk g√n√re automatiquement:


 RAPPORT TRIMESTRIEL - GESTION DES RISQUES
P√riode: Oct - D√c 
G√n√r√ le:  D√cembre 

. R√âSUM√â EX√âCUTIF
     risques identifi√s
     risques r√solus ce trimestre (-%)
     mitigations en cours (deadline: Q )
      risques Critiques remont√s √† la Direction

. TENDANCES
   [Graphique] √âvolution du nombre de risques
   - Trend: ‚Üì -% vs Q (Positif!)
   - R√solutions:  risques
   - Nouveaux:  risques

. D√âTAIL PAR DOMAINE
   
   Infrastructure:  risques
   ‚îú‚îÄ Critiques:  (Vieux serveur Windows XP)
   ‚îú‚îÄ Hauts: 
   ‚îî‚îÄ Moyens: 

   Application:  risques
   ‚îú‚îÄ Critiques:  (D√pendances outdated)
   ‚îú‚îÄ Hauts: 
   ‚îî‚îÄ Moyens: 

   Data & Compliance:  risques
   ‚îú‚îÄ Critiques: 
   ‚îú‚îÄ Hauts: 
   ‚îî‚îÄ Moyens: 

. MITIGATIONS EN COURS
   
    Upgrade Node.js (% complete)
      ‚îî‚îÄ Deadline:  Jan 
   
    Impl√menter MFA (% complete)
      ‚îî‚îÄ Deadline:  Feb 
   
    Audit s√curit√ externe (% complete)
      ‚îî‚îÄ Deadline:  Mar 

. CONFORMIT√â
   ISO :  % couvert (vs % Q)
   RGPD:  % couvert
   SOC:  % en cours

. RECOMMANDATIONS
   - Acc√l√rer l'upgrade Node.js (Critique)
   - Impl√menter MFA imm√diatement (S√curit√)
   - Refondre l'architecture legacy (Moyen terme)

---
Sign√ num√riquement par OpenRisk v..


 ‚É£ Exporter le Rapport

Depuis OpenRisk:
bash
 Interface: Reports ‚Üí Download Trimestral Report
 Formats disponibles:
 - PDF (pr√™t √† imprimer)
 - Excel (pour analyse)
 - JSON (pour BI tools)


 ‚É£ Temps N√cessaire

Avant:  jours (collecte manuelle + mise en forme)

Jour : Envoyer des mails aux √quipes
Jour -: Collecter les r√ponses
Jour : Formatter en PowerPoint
Jour : Validation + corrections


Avec OpenRisk:  minutes

. Click "Generate Quarterly Report"
. T√l√charger PDF
. Envoyer aux stakeholders


  Impact R√el
| Avant | Apr√s |
|-------|-------|
|  jours/mois de pr√paration |  min/trimestre |
| Donn√es potentiellement outdated | Donn√es en temps r√el |
| Impossible de tracker l'√volution | Graphiques de tendance |
| Format varie chaque fois | Format coh√rent & professionnel |

R√sultat: Le RSSI peut justifier son budget aupr√s de la direction avec donn√es pr√cises.

---

  Synth√se: Pourquoi OpenRisk?

 Pour les Startups
 Automatiser = moins de temps manuel  
 Prioriser = focuser sur ce qui compte  
 √âcheller = passer de  √†  risques facilement

 Pour les PME
 Centraliser = une source de v√rit√  
 Int√grer = connecter outils existants  
 Rapporter = prouver la s√curit√

 Pour les Entreprises
 Automatiser = √conomiser + jours/an par RSSI  
 Auditer = rapports conformit√ en  min  
 Gouverner = visibilit√ compl√te pour la direction

---

 üìû Pr√™t √† essayer?

[‚Üí D√marrer en  minutes](QUICK_ONBOARDING.md)

Des questions? Consultez [API_REFERENCE.md](API_REFERENCE.md) ou ouvrez une [discussion](https://github.com/alex-dembele/OpenRisk/discussions).
