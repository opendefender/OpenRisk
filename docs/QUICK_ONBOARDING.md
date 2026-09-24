# OpenRisk - Guide de Démarrage Rapide

Bienvenue! Ce guide vous permet de **démarrer en 5 minutes** et d'explorer OpenRisk avec des données réalistes.

---

## ⚡ Étape 1: Démarrer le Système (2 min)

### Prérequis
- Docker & Docker Compose installés
- Git
- Un terminal (Bash, Zsh, PowerShell, etc.)

### Lancer OpenRisk

```bash
# 1. Cloner le repo
git clone https://github.com/alex-dembele/OpenRisk.git
cd OpenRisk

# 2. Démarrer tous les services
docker compose up -d

# 3. Vérifier que tout fonctionne
docker compose ps
# Devrait afficher: db, redis, backend, frontend (tous UP)

# 4. Accéder à l'interface
# → Frontend: http://localhost:5173
# → API Backend: http://localhost:8080
```

### ✅ Contrôle de Santé

```bash
# Vérifier que les services répondent
curl http://localhost:8080/health
# Résultat attendu: {"status":"healthy"}
```

---

## 🔐 Étape 2: Se Connecter (1 min)

### Premier accès
```
📧 Email: admin@opendefender.io
🔑 Mot de passe: celui affiché par install.sh, ou INITIAL_ADMIN_PASSWORD
```
Il n'existe aucun mot de passe par défaut. Sans `INITIAL_ADMIN_PASSWORD`, le
backend en génère un au premier démarrage et l'écrit dans
`/app/secrets/initial_admin_password`.

### Première Connexion

1. Ouvrir http://localhost:5173 dans votre navigateur
2. Entrer les identifiants ci-dessus
3. Cliquer "Login"

**Vous arrivez sur le Dashboard!**

---

## 📊 Étape 3: Explorer le Dashboard (30 sec)

Vous voyez 4 sections:

### 📈 Haut Gauche: Vue d'Ensemble
```
8 Risques Hauts
12 Risques Moyens
5 Risques Bas
```

### 📉 Haut Droit: Graphique de Tendance
```
Montre l'évolution des risques sur les 30 derniers jours
(Actuellement vide, on va ajouter des données)
```

### 🗺️ Bas Gauche: Heatmap
```
Matrice de probabilité vs impact
Permet de visualiser les risques visuellement
```

### 📋 Bas Droit: Risques Récents
```
Liste des derniers risques créés
(Actuellement vide)
```

---

## 📥 Étape 4: Importer des Données de Test (2 min)

### Option A: Importer via API (Recommandé)

**Télécharger le fichier de test:**

```bash
# Le fichier est inclus dans le repo
cat dev/fixtures/risks.json
```

**Importer les données:**

```bash
# Option 1: Via cURL (ligne de commande)
curl -X POST http://localhost:8080/api/risks/bulk-import \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d @dev/fixtures/risks.json

# Option 2: Via l'interface (plus simple)
# 1. Aller à Settings → Data Management
# 2. Cliquer "Import Data"
# 3. Télécharger dev/fixtures/risks.json
# 4. Cliquer "Import"
```

### Option B: Créer Manuellement un Risque

1. Cliquer sur "Risks" dans le menu
2. Cliquer "Create Risk"
3. Remplir le formulaire:

```
Titre: Vulnérabilité SQL Injection dans formulaire login
Description: L'input utilisateur n'est pas échappé
Framework: OWASP Top 10 - A03:2021 Injection
Criticité: Haute
Probabilité: Moyenne
Status: Identifié

Score Calculé Automatiquement: 7.5/10 ✅
```

4. Cliquer "Save"

---

## 🛡️ Étape 5: Créer une Mitigation (2 min)

### Depuis un Risque Existant

1. Cliquer sur un risque (ex: "Vulnérabilité SQL")
2. Aller à l'onglet "Mitigations"
3. Cliquer "Add Mitigation"
4. Remplir:

```
Titre: Utiliser des Prepared Statements
Description: Refactoriser la couche base de données
Status: In Progress
Owner: Backend Team Lead
Deadline: 15 Janvier 2026
```

### Ajouter des Sous-Actions (Checklist)

```
Sub-actions:
☐ Valider avec l'équipe sécurité
☐ Écrire les tests unitaires
☐ Déployer en staging
☐ Tester 24h en prod
☐ Monitorer les logs
```

**Cocher au fur et à mesure:**
```bash
# Quand l'action est faite, cliquer la case ☐ → ☑️
# Le système track automatiquement la progression
```

---

## 📊 Étape 6: Générer un Rapport (1 min)

### Créer un Rapport Simple

1. Cliquer "Reports" dans le menu
2. Cliquer "Create Report"
3. Sélectionner:
   - **Type**: Risk Summary
   - **Période**: Ce mois-ci
   - **Format**: PDF
4. Cliquer "Generate"

**Le rapport est généré en 10 secondes!**

### Ce qu'on Voit dans le Rapport

```
📊 RAPPORT DE GESTION DES RISQUES
Généré le: 22 Décembre 2025

Résumé:
- Total risques: 3
- Critiques: 1
- Hauts: 1
- Moyens: 1

Détail:
1. Vulnérabilité SQL (Score: 7.5) → Mitigation en cours
2. ...

Actions Recommandées:
- Accélérer la mitigation Critique
- ...
```

---

## 🔌 Étape 7: Connecter vos Outils (Optionnel)

### Splunk Integration

Si vous utilisez Splunk pour la sécurité:

```bash
# 1. Aller à Settings → Integrations
# 2. Cliquer "Add Integration"
# 3. Sélectionner "Splunk"
# 4. Entrer:
   SPLUNK_URL=https://splunk.votreentreprise.com:8089
   SPLUNK_API_TOKEN=xxxxxxxxxxxxx
   IMPORT_ALERTS=true
# 5. Cliquer "Test Connection"
# 6. Cliquer "Enable"
```

Après activation, les alertes Splunk s'importeront automatiquement dans OpenRisk!

### TheHive Integration

Si vous utilisez TheHive pour les incidents:

```bash
# Settings → Integrations → TheHive
   THEHIVE_URL=https://thehive.votreentreprise.com
   THEHIVE_API_KEY=xxxxxxxxxxxxx
# Synchronisation bi-directionnelle activée!
```

---

## 📝 Étape 8: Inviter des Utilisateurs (Optionnel)

### Ajouter un Membre de l'Équipe

1. Aller à "Settings" → "Team"
2. Cliquer "Invite User"
3. Entrer l'email: `john@votreentreprise.com`
4. Sélectionner le rôle:
   ```
   - Admin: Accès complet
   - Risk Manager: Créer/modifier risques
   - Analyst: Voir & commenter
   - Viewer: Lecture seule
   ```
5. Cliquer "Send Invite"

L'utilisateur recevra un email d'invitation!

---

## 🎯 Commandes Utiles

### Vérifier l'État

```bash
# Est-ce que tout fonctionne?
docker compose ps

# Voir les logs
docker compose logs backend
docker compose logs frontend

# Redémarrer les services
docker compose restart
```

### Arrêter / Redémarrer

```bash
# Arrêter
docker compose down

# Arrêter et effacer les données
docker compose down -v

# Redémarrer
docker compose up -d
```

### Réinitialiser les Données de Test

```bash
# Effacer et recommencer zéro
docker compose down -v
docker compose up -d
# Puis importer les données (Étape 4)
```

---

## 🚨 Troubleshooting

### "Connection refused" sur localhost:5173

```bash
# Le frontend n'a pas démarré
# Solution:
docker compose restart frontend
docker compose logs frontend  # Voir l'erreur

# Ou attendre 30 secondes, Docker est lent au premier démarrage
```

### "Database connection error"

```bash
# La base de données n'est pas prête
# Solution:
docker compose logs db  # Vérifier les logs

# Ou:
docker compose down -v
docker compose up -d
```

### "Impossible de se connecter en admin@opendefender.io"

```bash
# Le mot de passe du premier admin est refusé
# (c'est INITIAL_ADMIN_PASSWORD, ou le fichier ci-dessous ; il n'y a pas de défaut)
docker compose exec backend cat /app/secrets/initial_admin_password
# Solution:
# 1. Vérifier que le backend est bien démarré
docker compose ps | grep backend
# Doit être "UP"

# 2. Vérifier les migrations sont appliquées
docker compose logs backend | grep "migration"

# 3. Réinitialiser complet
docker compose down -v
docker compose up -d
# Attendre 30 secondes

# 4. Réessayer
```

### Port 5173 déjà utilisé

```bash
# Un autre processus utilise le port
# Solution:

# Option 1: Chercher le processus
lsof -i :5173
kill -9 <PID>

# Option 2: Utiliser un autre port
docker compose down
# Modifier docker-compose.yaml ligne frontend:
#   ports:
#     - "5174:5173"  # ← Changer 5173 en 5174
docker compose up -d

# Accéder à http://localhost:5174
```

---

## 📚 Prochaines Étapes

### Pour Aller Plus Loin

1. **Lire les cas d'usage réels**: [USE_CASES.md](USE_CASES.md)
2. **Explorer l'API complète**: [API_REFERENCE.md](API_REFERENCE.md)
3. **Configurer SSO**: [SAML_OAUTH2_INTEGRATION.md](SAML_OAUTH2_INTEGRATION.md)
4. **Déployer en Production**: [PRODUCTION_RUNBOOK.md](PRODUCTION_RUNBOOK.md)
5. **Intégrer vos outils**: [SYNC_ENGINE.md](SYNC_ENGINE.md)

### Documentation Recommandée

| Doc | Pour Qui | Temps |
|-----|----------|-------|
| [USE_CASES.md](USE_CASES.md) | Découvrir la valeur réelle | 5 min |
| [API_REFERENCE.md](API_REFERENCE.md) | Développeurs & API | 10 min |
| [SAML_OAUTH2_INTEGRATION.md](SAML_OAUTH2_INTEGRATION.md) | IT & Admins | 15 min |
| [LOCAL_DEVELOPMENT.md](LOCAL_DEVELOPMENT.md) | Contribuer au projet | 20 min |

---

## ❓ Questions?

- 💬 **Chat**: [GitHub Discussions](https://github.com/alex-dembele/OpenRisk/discussions)
- 🐛 **Bug**: [Ouvrir une Issue](https://github.com/alex-dembele/OpenRisk/issues)
- 📖 **Docs**: [Voir tous les guides](./README.md)

---

## 🎉 Bravo!

Vous venez de mettre en place une **plateforme de gestion des risques complète** en 5 minutes!

**Prochaine étape?** → Lire [USE_CASES.md](USE_CASES.md) pour voir comment l'utiliser pour votre équipe 🚀
