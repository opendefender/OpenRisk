 OpenRisk - Guide de Dmarrage Rapide

Bienvenue! Ce guide vous permet de dmarrer en  minutes et d'explorer OpenRisk avec des donnes ralistes.

---

  Étape : Dmarrer le Systme ( min)

 Prrequis
- Docker & Docker Compose installs
- Git
- Un terminal (Bash, Zsh, PowerShell, etc.)

 Lancer OpenRisk

bash
 . Cloner le repo
git clone https://github.com/alex-dembele/OpenRisk.git
cd OpenRisk

 . Dmarrer tous les services
docker compose up -d

 . Vrifier que tout fonctionne
docker compose ps
 Devrait afficher: db, redis, backend, frontend (tous UP)

 . Accder à l'interface
 → Frontend: http://localhost:
 → API Backend: http://localhost:


  Contrle de Sant

bash
 Vrifier que les services rpondent
curl http://localhost:/health
 Rsultat attendu: {"status":"healthy"}


---

  Étape : Se Connecter ( min)

 Identifiants par dfaut

 Email: admin@openrisk.local
 Mot de passe: admin


 Premire Connexion

. Ouvrir http://localhost: dans votre navigateur
. Entrer les identifiants ci-dessus
. Cliquer "Login"

Vous arrivez sur le Dashboard!

---

  Étape : Explorer le Dashboard ( sec)

Vous voyez  sections:

  Haut Gauche: Vue d'Ensemble

 Risques Hauts
 Risques Moyens
 Risques Bas


  Haut Droit: Graphique de Tendance

Montre l'volution des risques sur les  derniers jours
(Actuellement vide, on va ajouter des donnes)


  Bas Gauche: Heatmap

Matrice de probabilit vs impact
Permet de visualiser les risques visuellement


  Bas Droit: Risques Rcents

Liste des derniers risques crs
(Actuellement vide)


---

  Étape : Importer des Donnes de Test ( min)

 Option A: Importer via API (Recommand)

Tlcharger le fichier de test:

bash
 Le fichier est inclus dans le repo
cat dev/fixtures/risks.json


Importer les donnes:

bash
 Option : Via cURL (ligne de commande)
curl -X POST http://localhost:/api/risks/bulk-import \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d @dev/fixtures/risks.json

 Option : Via l'interface (plus simple)
 . Aller à Settings → Data Management
 . Cliquer "Import Data"
 . Tlcharger dev/fixtures/risks.json
 . Cliquer "Import"


 Option B: Crer Manuellement un Risque

. Cliquer sur "Risks" dans le menu
. Cliquer "Create Risk"
. Remplir le formulaire:


Titre: Vulnrabilit SQL Injection dans formulaire login
Description: L'input utilisateur n'est pas chapp
Framework: OWASP Top  - A: Injection
Criticit: Haute
Probabilit: Moyenne
Status: Identifi

Score Calcul Automatiquement: ./ 


. Cliquer "Save"

---

  Étape : Crer une Mitigation ( min)

 Depuis un Risque Existant

. Cliquer sur un risque (ex: "Vulnrabilit SQL")
. Aller à l'onglet "Mitigations"
. Cliquer "Add Mitigation"
. Remplir:


Titre: Utiliser des Prepared Statements
Description: Refactoriser la couche base de donnes
Status: In Progress
Owner: Backend Team Lead
Deadline:  Janvier 


 Ajouter des Sous-Actions (Checklist)


Sub-actions:
 Valider avec l'quipe scurit
 Écrire les tests unitaires
 Dployer en staging
 Tester h en prod
 Monitorer les logs


Cocher au fur et à mesure:
bash
 Quand l'action est faite, cliquer la case  → 
 Le systme track automatiquement la progression


---

  Étape : Gnrer un Rapport ( min)

 Crer un Rapport Simple

. Cliquer "Reports" dans le menu
. Cliquer "Create Report"
. Slectionner:
   - Type: Risk Summary
   - Priode: Ce mois-ci
   - Format: PDF
. Cliquer "Generate"

Le rapport est gnr en  secondes!

 Ce qu'on Voit dans le Rapport


 RAPPORT DE GESTION DES RISQUES
Gnr le:  Dcembre 

Rsum:
- Total risques: 
- Critiques: 
- Hauts: 
- Moyens: 

Dtail:
. Vulnrabilit SQL (Score: .) → Mitigation en cours
. ...

Actions Recommandes:
- Acclrer la mitigation Critique
- ...


---

  Étape : Connecter vos Outils (Optionnel)

 Splunk Integration

Si vous utilisez Splunk pour la scurit:

bash
 . Aller à Settings → Integrations
 . Cliquer "Add Integration"
 . Slectionner "Splunk"
 . Entrer:
   SPLUNK_URL=https://splunk.votreentreprise.com:
   SPLUNK_API_TOKEN=xxxxxxxxxxxxx
   IMPORT_ALERTS=true
 . Cliquer "Test Connection"
 . Cliquer "Enable"


Aprs activation, les alertes Splunk s'importeront automatiquement dans OpenRisk!

 TheHive Integration

Si vous utilisez TheHive pour les incidents:

bash
 Settings → Integrations → TheHive
   THEHIVE_URL=https://thehive.votreentreprise.com
   THEHIVE_API_KEY=xxxxxxxxxxxxx
 Synchronisation bi-directionnelle active!


---

  Étape : Inviter des Utilisateurs (Optionnel)

 Ajouter un Membre de l'Équipe

. Aller à "Settings" → "Team"
. Cliquer "Invite User"
. Entrer l'email: john@votreentreprise.com
. Slectionner le rle:
   
   - Admin: Accs complet
   - Risk Manager: Crer/modifier risques
   - Analyst: Voir & commenter
   - Viewer: Lecture seule
   
. Cliquer "Send Invite"

L'utilisateur recevra un email d'invitation!

---

  Commandes Utiles

 Vrifier l'État

bash
 Est-ce que tout fonctionne?
docker compose ps

 Voir les logs
docker compose logs backend
docker compose logs frontend

 Redmarrer les services
docker compose restart


 Arrêter / Redmarrer

bash
 Arrêter
docker compose down

 Arrêter et effacer les donnes
docker compose down -v

 Redmarrer
docker compose up -d


 Rinitialiser les Donnes de Test

bash
 Effacer et recommencer zro
docker compose down -v
docker compose up -d
 Puis importer les donnes (Étape )


---

  Troubleshooting

 "Connection refused" sur localhost:

bash
 Le frontend n'a pas dmarr
 Solution:
docker compose restart frontend
docker compose logs frontend   Voir l'erreur

 Ou attendre  secondes, Docker est lent au premier dmarrage


 "Database connection error"

bash
 La base de donnes n'est pas prête
 Solution:
docker compose logs db   Vrifier les logs

 Ou:
docker compose down -v
docker compose up -d


 "Can't login with admin@openrisk.local"

bash
 Les credentials par dfaut ne fonctionnent pas
 Solution:
 . Vrifier que le backend est bien dmarr
docker compose ps | grep backend
 Doit être "UP"

 . Vrifier les migrations sont appliques
docker compose logs backend | grep "migration"

 . Rinitialiser complet
docker compose down -v
docker compose up -d
 Attendre  secondes

 . Ressayer


 Port  djà utilis

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
     - ":"   ← Changer  en 
docker compose up -d

 Accder à http://localhost:


---

  Prochaines Étapes

 Pour Aller Plus Loin

. Lire les cas d'usage rels: [USE_CASES.md](USE_CASES.md)
. Explorer l'API complte: [API_REFERENCE.md](API_REFERENCE.md)
. Configurer SSO: [SAML_OAUTH_INTEGRATION.md](SAML_OAUTH_INTEGRATION.md)
. Dployer en Production: [PRODUCTION_RUNBOOK.md](PRODUCTION_RUNBOOK.md)
. Intgrer vos outils: [SYNC_ENGINE.md](SYNC_ENGINE.md)

 Documentation Recommande

| Doc | Pour Qui | Temps |
|-----|----------|-------|
| [USE_CASES.md](USE_CASES.md) | Dcouvrir la valeur relle |  min |
| [API_REFERENCE.md](API_REFERENCE.md) | Dveloppeurs & API |  min |
| [SAML_OAUTH_INTEGRATION.md](SAML_OAUTH_INTEGRATION.md) | IT & Admins |  min |
| [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) | Contribuer au projet |  min |

---

  Questions?

-  Chat: [GitHub Discussions](https://github.com/alex-dembele/OpenRisk/discussions)
-  Bug: [Ouvrir une Issue](https://github.com/alex-dembele/OpenRisk/issues)
-  Docs: [Voir tous les guides](./README.md)

---

  Bravo!

Vous venez de mettre en place une plateforme de gestion des risques complte en  minutes!

Prochaine tape? → Lire [USE_CASES.md](USE_CASES.md) pour voir comment l'utiliser pour votre quipe 
