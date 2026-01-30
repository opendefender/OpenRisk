 OpenRisk - Guide de D√marrage Rapide

Bienvenue! Ce guide vous permet de d√marrer en  minutes et d'explorer OpenRisk avec des donn√es r√alistes.

---

 ‚ö° √âtape : D√marrer le Syst√me ( min)

 Pr√requis
- Docker & Docker Compose install√s
- Git
- Un terminal (Bash, Zsh, PowerShell, etc.)

 Lancer OpenRisk

bash
 . Cloner le repo
git clone https://github.com/alex-dembele/OpenRisk.git
cd OpenRisk

 . D√marrer tous les services
docker compose up -d

 . V√rifier que tout fonctionne
docker compose ps
 Devrait afficher: db, redis, backend, frontend (tous UP)

 . Acc√der √† l'interface
 ‚Üí Frontend: http://localhost:
 ‚Üí API Backend: http://localhost:


  Contr√le de Sant√

bash
 V√rifier que les services r√pondent
curl http://localhost:/health
 R√sultat attendu: {"status":"healthy"}


---

  √âtape : Se Connecter ( min)

 Identifiants par d√faut

üìß Email: admin@openrisk.local
 Mot de passe: admin


 Premi√re Connexion

. Ouvrir http://localhost: dans votre navigateur
. Entrer les identifiants ci-dessus
. Cliquer "Login"

Vous arrivez sur le Dashboard!

---

  √âtape : Explorer le Dashboard ( sec)

Vous voyez  sections:

 üìà Haut Gauche: Vue d'Ensemble

 Risques Hauts
 Risques Moyens
 Risques Bas


 üìâ Haut Droit: Graphique de Tendance

Montre l'√volution des risques sur les  derniers jours
(Actuellement vide, on va ajouter des donn√es)


 üó∫ Bas Gauche: Heatmap

Matrice de probabilit√ vs impact
Permet de visualiser les risques visuellement


  Bas Droit: Risques R√cents

Liste des derniers risques cr√√s
(Actuellement vide)


---

 üì• √âtape : Importer des Donn√es de Test ( min)

 Option A: Importer via API (Recommand√)

T√l√charger le fichier de test:

bash
 Le fichier est inclus dans le repo
cat dev/fixtures/risks.json


Importer les donn√es:

bash
 Option : Via cURL (ligne de commande)
curl -X POST http://localhost:/api/risks/bulk-import \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d @dev/fixtures/risks.json

 Option : Via l'interface (plus simple)
 . Aller √† Settings ‚Üí Data Management
 . Cliquer "Import Data"
 . T√l√charger dev/fixtures/risks.json
 . Cliquer "Import"


 Option B: Cr√er Manuellement un Risque

. Cliquer sur "Risks" dans le menu
. Cliquer "Create Risk"
. Remplir le formulaire:


Titre: Vuln√rabilit√ SQL Injection dans formulaire login
Description: L'input utilisateur n'est pas √chapp√
Framework: OWASP Top  - A: Injection
Criticit√: Haute
Probabilit√: Moyenne
Status: Identifi√

Score Calcul√ Automatiquement: ./ 


. Cliquer "Save"

---

  √âtape : Cr√er une Mitigation ( min)

 Depuis un Risque Existant

. Cliquer sur un risque (ex: "Vuln√rabilit√ SQL")
. Aller √† l'onglet "Mitigations"
. Cliquer "Add Mitigation"
. Remplir:


Titre: Utiliser des Prepared Statements
Description: Refactoriser la couche base de donn√es
Status: In Progress
Owner: Backend Team Lead
Deadline:  Janvier 


 Ajouter des Sous-Actions (Checklist)


Sub-actions:
‚òê Valider avec l'√quipe s√curit√
‚òê √âcrire les tests unitaires
‚òê D√ployer en staging
‚òê Tester h en prod
‚òê Monitorer les logs


Cocher au fur et √† mesure:
bash
 Quand l'action est faite, cliquer la case ‚òê ‚Üí ‚òë
 Le syst√me track automatiquement la progression


---

  √âtape : G√n√rer un Rapport ( min)

 Cr√er un Rapport Simple

. Cliquer "Reports" dans le menu
. Cliquer "Create Report"
. S√lectionner:
   - Type: Risk Summary
   - P√riode: Ce mois-ci
   - Format: PDF
. Cliquer "Generate"

Le rapport est g√n√r√ en  secondes!

 Ce qu'on Voit dans le Rapport


 RAPPORT DE GESTION DES RISQUES
G√n√r√ le:  D√cembre 

R√sum√:
- Total risques: 
- Critiques: 
- Hauts: 
- Moyens: 

D√tail:
. Vuln√rabilit√ SQL (Score: .) ‚Üí Mitigation en cours
. ...

Actions Recommand√es:
- Acc√l√rer la mitigation Critique
- ...


---

 üîå √âtape : Connecter vos Outils (Optionnel)

 Splunk Integration

Si vous utilisez Splunk pour la s√curit√:

bash
 . Aller √† Settings ‚Üí Integrations
 . Cliquer "Add Integration"
 . S√lectionner "Splunk"
 . Entrer:
   SPLUNK_URL=https://splunk.votreentreprise.com:
   SPLUNK_API_TOKEN=xxxxxxxxxxxxx
   IMPORT_ALERTS=true
 . Cliquer "Test Connection"
 . Cliquer "Enable"


Apr√s activation, les alertes Splunk s'importeront automatiquement dans OpenRisk!

 TheHive Integration

Si vous utilisez TheHive pour les incidents:

bash
 Settings ‚Üí Integrations ‚Üí TheHive
   THEHIVE_URL=https://thehive.votreentreprise.com
   THEHIVE_API_KEY=xxxxxxxxxxxxx
 Synchronisation bi-directionnelle activ√e!


---

  √âtape : Inviter des Utilisateurs (Optionnel)

 Ajouter un Membre de l'√âquipe

. Aller √† "Settings" ‚Üí "Team"
. Cliquer "Invite User"
. Entrer l'email: john@votreentreprise.com
. S√lectionner le r√le:
   
   - Admin: Acc√s complet
   - Risk Manager: Cr√er/modifier risques
   - Analyst: Voir & commenter
   - Viewer: Lecture seule
   
. Cliquer "Send Invite"

L'utilisateur recevra un email d'invitation!

---

  Commandes Utiles

 V√rifier l'√âtat

bash
 Est-ce que tout fonctionne?
docker compose ps

 Voir les logs
docker compose logs backend
docker compose logs frontend

 Red√marrer les services
docker compose restart


 Arr√™ter / Red√marrer

bash
 Arr√™ter
docker compose down

 Arr√™ter et effacer les donn√es
docker compose down -v

 Red√marrer
docker compose up -d


 R√initialiser les Donn√es de Test

bash
 Effacer et recommencer z√ro
docker compose down -v
docker compose up -d
 Puis importer les donn√es (√âtape )


---

  Troubleshooting

 "Connection refused" sur localhost:

bash
 Le frontend n'a pas d√marr√
 Solution:
docker compose restart frontend
docker compose logs frontend   Voir l'erreur

 Ou attendre  secondes, Docker est lent au premier d√marrage


 "Database connection error"

bash
 La base de donn√es n'est pas pr√™te
 Solution:
docker compose logs db   V√rifier les logs

 Ou:
docker compose down -v
docker compose up -d


 "Can't login with admin@openrisk.local"

bash
 Les credentials par d√faut ne fonctionnent pas
 Solution:
 . V√rifier que le backend est bien d√marr√
docker compose ps | grep backend
 Doit √™tre "UP"

 . V√rifier les migrations sont appliqu√es
docker compose logs backend | grep "migration"

 . R√initialiser complet
docker compose down -v
docker compose up -d
 Attendre  secondes

 . R√essayer


 Port  d√j√† utilis√

bash
 Un autre processus utilise le port
 Solution:

 Option : Chercher le processus
lsof -i :
kill - <PID>

 Option : Utiliser un autre port
docker compose down
 Modifier docker-compose.yaml ligne frontend:
   ports:
     - ":"   ‚Üê Changer  en 
docker compose up -d

 Acc√der √† http://localhost:


---

 üìö Prochaines √âtapes

 Pour Aller Plus Loin

. Lire les cas d'usage r√els: [USE_CASES.md](USE_CASES.md)
. Explorer l'API compl√te: [API_REFERENCE.md](API_REFERENCE.md)
. Configurer SSO: [SAML_OAUTH_INTEGRATION.md](SAML_OAUTH_INTEGRATION.md)
. D√ployer en Production: [PRODUCTION_RUNBOOK.md](PRODUCTION_RUNBOOK.md)
. Int√grer vos outils: [SYNC_ENGINE.md](SYNC_ENGINE.md)

 Documentation Recommand√e

| Doc | Pour Qui | Temps |
|-----|----------|-------|
| [USE_CASES.md](USE_CASES.md) | D√couvrir la valeur r√elle |  min |
| [API_REFERENCE.md](API_REFERENCE.md) | D√veloppeurs & API |  min |
| [SAML_OAUTH_INTEGRATION.md](SAML_OAUTH_INTEGRATION.md) | IT & Admins |  min |
| [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) | Contribuer au projet |  min |

---

 ‚ùì Questions?

- üí¨ Chat: [GitHub Discussions](https://github.com/alex-dembele/OpenRisk/discussions)
- üêõ Bug: [Ouvrir une Issue](https://github.com/alex-dembele/OpenRisk/issues)
- üìñ Docs: [Voir tous les guides](./README.md)

---

  Bravo!

Vous venez de mettre en place une plateforme de gestion des risques compl√te en  minutes!

Prochaine √tape? ‚Üí Lire [USE_CASES.md](USE_CASES.md) pour voir comment l'utiliser pour votre √quipe 
