# OpenRisk - Cas d'Usage Réels

Ce document présente 3 cas d'usage concrets où OpenRisk crée de la valeur immédiate.

---

## 📌 Cas 1: Startup SaaS - Mesurer & Prioriser les Risques Prod

### Le Problème
**TechStart.io** est une startup SaaS avec 50 employés et 2000 clients. Leur infrastructure grandit mais leur processus de gestion des risques est artisanal:
- Risques documentés dans Google Sheets
- Pas de scoring centralisé
- Les alertes sécurité s'accumulent sans priorisation
- CISO travaille 70h/semaine à tracker manuellement

### Solution avec OpenRisk

#### 1️⃣ Configuration Initiale (30 min)
```bash
# Démarrer OpenRisk
docker compose up -d

# Accéder à l'interface
# → http://localhost:5173
# Email: admin@openrisk.local | Password: admin123
```

#### 2️⃣ Créer les Catégories de Risques
Depuis l'interface:
- **Infrastructure** (serveurs, bases de données, réseaux)
- **Application** (bugs, vulnérabilités logicielles)
- **Data** (fuites, conformité RGPD)
- **Opérations** (incidents, RTO/RPO)

#### 3️⃣ Évaluer les Risques Existants
Exemple: **Vulnérabilité dans Node.js v18**

```
Titre: Vulnérabilité Node.js 18 - Injection HTTP
Description: Un attaquant peut envoyer des headers malveillants
Framework: OWASP Top 10 - Injection
Criticité: Haute (Availability)
Probabilité: Moyenne (besoin d'exploitation)

Score Automatique: 7.2/10 (Haute Priorité)
```

#### 4️⃣ Créer le Plan d'Atténuation
```
Mitigation: Upgrade Node.js 18 → 20 LTS
Status: En Cours
Responsable: DevOps Lead
Deadline: 15 janvier 2026

Sub-actions (Checklist):
☑️ Tester sur environnement staging
☑️ Valider les dépendances
☐ Déployer en prod
☐ Monitoring 48h après déploiement
```

#### 5️⃣ Dashboard Temps Réel
Le CISO voit en un coup d'œil:
- **8 risques Hauts** → Demandent action immédiate
- **12 risques Moyens** → À planifier
- **5 risques Bas** → À monitorer
- **Graphique de tendance** → Montre 3 risques résolus ce mois-ci

### 💡 Impact Réel
| Avant | Après |
|-------|-------|
| 70h/semaine de gestion manuelle | 5h/semaine de suivi |
| Pas de visibilité pour l'équipe exec | Dashboard en temps réel |
| Risques oubliés | 100% tracés |
| Rapports mensuels = urgence | Rapports générés en 2 clics |

**Résultat**: Le CISO peut se concentrer sur la stratégie au lieu de l'administratif.

---

## 📌 Cas 2: PME - Centraliser les Alertes Sécurité

### Le Problème
**SecureLogistics.fr** est une PME de 150 employés avec une infrastructure hybride:
- Serveurs on-premise + AWS
- Elastic Stack pour les logs
- Splunk pour la sécurité
- Les alertes arrivent partout: mail, Slack, tickets Jira
- Impossible de tracker "qui doit faire quoi"

### Solution avec OpenRisk

#### 1️⃣ Importer les Données Existantes
OpenRisk peut se connecter à vos outils existants:

```bash
# Configuration dans l'interface (Settings → Integrations)

# Option 1: Splunk Integration
API_SPLUNK_URL=https://splunk.securelog.fr:8089
API_SPLUNK_TOKEN=xxxxx
IMPORT_ALERTS=true

# Option 2: Elastic Integration  
ELASTICSEARCH_URL=https://elastic.securelog.fr:9200
IMPORT_ALERTS=true

# Option 3: Manuel (importer un CSV)
# Uploadez votre fichier dans OpenRisk
```

#### 2️⃣ Exemple: Alerte Splunk "Connexion SSH Brute-Force"

**L'alerte arrive:**
```
[CRITICAL] 47 tentatives SSH échouées sur srv-prod-01
Source: 203.0.113.45
Temps: 2025-12-22 14:32:00
```

**Dans OpenRisk:**
- Créer un Risque: "Attaque par force brute sur SSH"
- Scorer automatiquement: 8.5/10 (Critère: tentatives répétées + prod)
- Assigner à: Responsable Infrastructure
- Lier à Mitigation: "Implémenter fail2ban"
- Sub-actions:
  ```
  ☑️ Bloquer l'IP immédiatement
  ☐ Vérifier si accès granted
  ☐ Implémenter rate limiting
  ☐ Ajouter 2FA obligatoire
  ```

#### 3️⃣ Tableau de Bord Centralisé
Un seul endroit pour voir:
- 🔴 **Critiques actifs**: 3
- 🟠 **Hauts**: 7
- 🟡 **Moyens**: 15
- 🟢 **Bas**: 32
- **Graphique**: Tendance des 30 derniers jours

#### 4️⃣ Intégration Team
```
Slack Integration:
- Notification quand nouveau risque Critique
- Daily digest des 5 risques à traiter
- Rapport hebdomadaire
```

### 💡 Impact Réel
| Avant | Après |
|-------|-------|
| Alertes dispersées = beaucoup oubliées | 100% centralisé |
| 3-4h de temps pour chercher "où est l'alerte" | 30s pour retrouver l'info |
| Pas d'ordre de priorité | Score automatique qui trie |
| Responsabilités floues | Chaque risque a un proprio |

**Résultat**: Les alertes deviennent des actions tracées, plus du bruit.

---

## 📌 Cas 3: RSSI - Rapports Trimestriels Automatisés

### Le Problème
**MegatechCorp.com** est une grande entreprise avec 500 employés. Le RSSI doit:
- Produire un rapport de conformité **chaque trimestre**
- Montrer les risques identifiés
- Prouver que les mitigations avancent
- Remettre à la direction + auditeurs externes
- Actuellement: **5 jours de travail** par rapport

### Solution avec OpenRisk

#### 1️⃣ Configuration Annuelle (1 heure)

```bash
# Dans Settings → Organization
Compliance_Framework: ISO 27001
Report_Frequency: Trimestrel
Auto_Export_Format: PDF + Excel
Recipients: 
  - direction@megatech.fr
  - audit@megatech.fr
  - ciso@megatech.fr
```

#### 2️⃣ Exemple de Rapport Q4 2025

**OpenRisk génère automatiquement:**

```
📊 RAPPORT TRIMESTRIEL - GESTION DES RISQUES
Période: Oct - Déc 2025
Généré le: 22 Décembre 2025

1. RÉSUMÉ EXÉCUTIF
   ✅ 47 risques identifiés
   ✅ 12 risques résolus ce trimestre (-20%)
   ✅ 8 mitigations en cours (deadline: Q1 2026)
   ⚠️  3 risques Critiques remontés à la Direction

2. TENDANCES
   [Graphique] Évolution du nombre de risques
   - Trend: ↓ -15% vs Q3 (Positif!)
   - Résolutions: 12 risques
   - Nouveaux: 8 risques

3. DÉTAIL PAR DOMAINE
   
   Infrastructure: 15 risques
   ├─ Critiques: 1 (Vieux serveur Windows XP)
   ├─ Hauts: 3
   └─ Moyens: 11

   Application: 18 risques
   ├─ Critiques: 2 (Dépendances outdated)
   ├─ Hauts: 5
   └─ Moyens: 11

   Data & Compliance: 14 risques
   ├─ Critiques: 0
   ├─ Hauts: 4
   └─ Moyens: 10

4. MITIGATIONS EN COURS
   
   ✅ Upgrade Node.js (70% complete)
      └─ Deadline: 15 Jan 2026
   
   ✅ Implémenter MFA (50% complete)
      └─ Deadline: 28 Feb 2026
   
   ✅ Audit sécurité externe (30% complete)
      └─ Deadline: 31 Mar 2026

5. CONFORMITÉ
   ISO 27001: ✅ 92% couvert (vs 85% Q3)
   RGPD: ✅ 100% couvert
   SOC2: ✅ 88% en cours

6. RECOMMANDATIONS
   - Accélérer l'upgrade Node.js (Critique)
   - Implémenter MFA immédiatement (Sécurité)
   - Refondre l'architecture legacy (Moyen terme)

---
Signé numériquement par OpenRisk v1.0.4
```

#### 2️⃣ Exporter le Rapport

**Depuis OpenRisk:**
```bash
# Interface: Reports → Download Trimestral Report
# Formats disponibles:
# - PDF (prêt à imprimer)
# - Excel (pour analyse)
# - JSON (pour BI tools)
```

#### 3️⃣ Temps Nécessaire

**Avant**: 5 jours (collecte manuelle + mise en forme)
```
Jour 1: Envoyer des mails aux équipes
Jour 2-3: Collecter les réponses
Jour 4: Formatter en PowerPoint
Jour 5: Validation + corrections
```

**Avec OpenRisk**: 10 minutes
```
1. Click "Generate Quarterly Report"
2. Télécharger PDF
3. Envoyer aux stakeholders
```

### 💡 Impact Réel
| Avant | Après |
|-------|-------|
| 5 jours/mois de préparation | 30 min/trimestre |
| Données potentiellement outdated | Données en temps réel |
| Impossible de tracker l'évolution | Graphiques de tendance |
| Format varie chaque fois | Format cohérent & professionnel |

**Résultat**: Le RSSI peut justifier son budget auprès de la direction avec données précises.

---

## 🎯 Synthèse: Pourquoi OpenRisk?

### Pour les Startups
✅ Automatiser = moins de temps manuel  
✅ Prioriser = focuser sur ce qui compte  
✅ Écheller = passer de 10 à 1000 risques facilement

### Pour les PME
✅ Centraliser = une source de vérité  
✅ Intégrer = connecter outils existants  
✅ Rapporter = prouver la sécurité

### Pour les Entreprises
✅ Automatiser = économiser 100+ jours/an par RSSI  
✅ Auditer = rapports conformité en 10 min  
✅ Gouverner = visibilité complète pour la direction

---

## 📞 Prêt à essayer?

**[→ Démarrer en 5 minutes](QUICK_ONBOARDING.md)**

Des questions? Consultez [API_REFERENCE.md](API_REFERENCE.md) ou ouvrez une [discussion](https://github.com/alex-dembele/OpenRisk/discussions).
