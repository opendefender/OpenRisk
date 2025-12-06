# 🎮 Guide d'Utilisation - Gamification UI

## Accéder à Your Gamification Profile

### Chemin Utilisateur
```
Navigation Sidebar
  └── ⚙️ Settings
      └── General Tab
          └── 🎮 Your Gamification Profile (NEW!)
```

### Visuellement
1. Cliquez sur **⚙️ Settings** dans la sidebar
2. Assurez-vous que l'onglet **General** est sélectionné
3. Scrollez vers le bas, vous verrez **"🎮 Your Gamification Profile"**

---

## Composants Affichés

### 1️⃣ Level Card (Cercle Principal)
```
┌─────────────────────────────────────┐
│      Level Card Premium             │
│  ┌───────────────────────────┐      │
│  │                           │      │
│  │       Circle Badge        │      │
│  │         Level 2           │      │
│  │                           │      │
│  └───────────────────────────┘      │
│      (Gradient dynamique)           │
│                                     │
│  Progression XP                     │
│  150 / 400 XP                       │
│  ▰▰▰▰▰▯▯▯▯▯ 37.5%                   │
│  Vers niveau 3                      │
└─────────────────────────────────────┘
```

**Couleurs par Niveau**:
- Niveau 1: 🟢 Green → Teal
- Niveau 2: 🔵 Blue → Cyan
- Niveau 3: 🟣 Purple → Indigo
- Niveau 4: 🩷 Pink → Rose
- Niveau 5+: 🟠 Orange → Red

### 2️⃣ Achievement Stats (Compteurs)
```
┌──────────────────────────────────┐
│  5               │      3        │
│  Risques Gérés   │  Atténuations │
└──────────────────────────────────┘
```

### 3️⃣ Badges Section
```
Badges Débloqués (4)

┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
│  ★      │  │  ★      │  │  ◇      │  │  ◇      │
│ Flag    │  │ Shield  │  │ Brain   │  │ Crown   │
│Initiator│  │Guardian │  │Strategist│ │ Legend  │
└─────────┘  └─────────┘  └─────────┘  └─────────┘

★ = Déverrouillé (Couleur jaune/or)
◇ = Verrouillé (Couleur grise)

Hover sur badge → Affiche description
```

---

## Système de Badges

### Les 4 Badges

| Badge | Nom | Description | Condition |
|-------|-----|-------------|-----------|
| 🚩 | **Initiator** | Créer votre premier risque | 1+ risque créé |
| 🛡️ | **Guardian** | Atténuer 5 risques | 5+ mitigations complétées |
| 🧠 | **Strategist** | Gérer plus de 10 risques | 10+ risques gérés |
| 👑 | **Legend** | Atteindre 1000 XP | 1000 XP ou plus |

### Comment Débloquer les Badges

```
INITIATOR (Démarrage)
└─ Action: Créer votre 1er risque
   🎯 Dashboard > "+ New Risk"
   ✓ Remplir titre + description
   ✓ Sélectionner assets (optionnel)
   ✓ Valider

GUARDIAN (Protection)
└─ Action: Compléter 5 mitigations
   🎯 Ajouter mitigation à risque
   ✓ Détails du risque > "+ Add Mitigation"
   ✓ Cocher "DONE" quand terminée
   × 5 fois = Badge!

STRATEGIST (Stratégie)
└─ Action: Gérer 10+ risques
   🎯 Créer progressivement
   📊 Dashboard affiche votre compte

LEGEND (Maîtrise)
└─ Action: Accumuler 1000 XP
   📈 +10 XP par risque créé
   📈 +50 XP par mitigation complétée
   🎯 (~20 risques + 10 mitigations)
```

---

## XP & Système de Progression

### Formule de Calcul

```
XP = (Nombre Risques × 10) + (Mitigations Complétées × 50)

Exemple:
- 5 risques créés      = 50 XP
- 3 mitigations faites = 150 XP
- TOTAL               = 200 XP → Niveau 2
```

### Progression de Niveau

```
Niveau 1: 0-99 XP        (Base)
Niveau 2: 100-399 XP     (Intermédiaire)
Niveau 3: 400-899 XP     (Avancé)
Niveau 4: 900-1599 XP    (Expert)
Niveau 5: 1600+ XP       (Maître)

Formule: Level = √(XP/100) + 1
```

### Barre de Progression

La barre affiche votre progression vers le niveau suivant:

```
Exemple: Niveau 2, 150/400 XP

[████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░] 37.5%
```

---

## Interactions Utilisateur

### Animations

1. **Chargement Initial**
   - Skeleton loader animé
   - Durée: ~1 seconde

2. **Barre XP**
   - Animée au montage (0 → X%)
   - Durée: 0.8 secondes
   - Easing: easeOut

3. **Badges**
   - Apparaissent en cascade (décalé)
   - Chaque badge: délai + 0.1s
   - Glow effect si déverrouillé

4. **Level Circle**
   - Pop animation (scale)
   - Spring physics

### Hover Effects

```
Hover sur un Badge:
  • Border s'illumine (couleur niveau)
  • Tooltip apparaît (description)
  • Légère animation scale

Hover sur compteur stats:
  • Background s'éclaircie
  • Cursor devient pointer
```

### États Affichés

```
✅ SUCCESS (Chargement ok)
   → Affiche tous les éléments

⏳ LOADING
   → Skeleton placeholder
   → Spinner (implicite via animate-pulse)

❌ ERROR
   → Icon AlertCircle rouge
   → Message d'erreur lisible
   → Bouton retry (manual F5)
```

---

## Exemples de Scénarios

### Scénario 1: Utilisateur Nouveau

```
1. Premier login
2. Va à Settings > General
3. Voit: "Vous avez 0 risques, 0 mitigations"
4. Level 1, 0 XP, 0% progression
5. 0 badges déverrouillés
6. → Invite à créer son 1er risque
```

**Action**: Retour Dashboard > "+ New Risk"

---

### Scénario 2: Utilisateur Actif

```
1. 15 risques créés, 8 mitigations complétées
2. XP = (15×10) + (8×50) = 550 XP
3. Level = √(550/100) + 1 ≈ Level 3
4. Progression vers Level 4: 550/900 = 61%
5. Badges:
   ✓ Initiator (1+ risque)
   ✓ Guardian (8 mitigations)
   ✓ Strategist (15 risques)
   ◇ Legend (besoin 1000 XP)
```

**Prochaine étape**: 450 XP manquants pour Legend

---

### Scénario 3: User Complète une Mitigation

```
AVANT:
└─ 10 risques, 4 mitigations, Level 1, 300 XP

ACTION: Compléter 1 mitigation

APRÈS (après refresh):
└─ 10 risques, 5 mitigations, Level 2, 350 XP
   ✓ Guardian badge déverrouille! 🎉
   └─ Toast notification: "Guardian Badge Unlocked!"
```

---

## Refresh & Mise à Jour

### Auto-Refresh
- ❌ Pas d'auto-refresh actuellement
- ✅ Refresh manuel: F5 ou Reload Page

### Mise à Jour après Action
1. Créer risque → Dashboard retour
2. Statut ne s'update pas automatiquement
3. **Solution**: Aller Settings > General (fetch effectué)
4. Ou: F5 pour refresh complet

### Prochainement (Backlog)
- 🔄 WebSocket events pour live update
- 🔔 Toast "XP Earned +10" quand risque créé
- 📊 Real-time stats refresh

---

## Troubleshooting

### "Impossible de charger les statistiques"

**Causes Possibles**:
1. JWT Token expiré
   → Solution: Logout > Reconnexion
2. Backend non accessible
   → Solution: Vérifier docker-compose up
3. Mauvais CORS
   → Solution: Vérifier allowOrigins dans main.go

**Vérifier**:
```bash
# Terminal 1: Backend
docker-compose up

# Terminal 2: Vérifier API
curl -H "Authorization: Bearer YOUR_JWT" \
  http://localhost:8080/api/v1/gamification/me
```

---

### Badges ne s'affichent pas

**Cause**: Icons non mappées (backend retourne icon name non géré)

**Solution**: Ajouter le mapping dans `getBadgeIcon()`:
```typescript
const icons: Record<string, React.ReactNode> = {
  Flag: <Target className="w-5 h-5" />,
  // Ajouter ici si besoin:
  NewIcon: <NewIconComponent className="w-5 h-5" />,
};
```

---

### XP ne s'update pas après création risque

**Cause**: Pas d'auto-refresh

**Solution Immédiate**:
- Appuyez sur F5
- Ou: Allez Settings > General (trigger fetch)

**Solution Future**: Implémentation WebSocket

---

## Règles et Contraintes

### Règles de Gamification

1. **XP**
   - S'accumule, ne diminue jamais
   - +10 XP par risque créé
   - +50 XP par mitigation complétée
   - Seul l'utilisateur voit ses stats

2. **Level**
   - Basé sur XP total
   - Formule quadratique
   - Max visible: 5+ (peut dépasser)

3. **Badges**
   - Une fois déverrouillés, ne peuvent pas être perdus
   - Conditions permanentes

4. **Privacy**
   - Chaque user ne voit que ses stats
   - Pas de leaderboard publique (futur)

---

## Intégration avec Workflow

### Flux Typique de Travail

```
1. LOGIN
   └─ Directed to Dashboard

2. CREATE RISK (Dashboard)
   └─ "+10 XP"
   └─ Stats mises à jour (après refresh)

3. ADD MITIGATION (Risk Details)
   └─ Statut = "DONE"
   └─ "+50 XP"

4. CHECK PROGRESS (Settings > General)
   └─ Voir progression XP
   └─ Voir badges déverrouillés
   └─ Motivation pour continuer

5. REPEAT
   └─ Plus on gère de risques
   └─ Plus on monte de niveau
   └─ Plus on déverrouille de badges
```

---

## Support & Contacte

- 📧 **Backend Issues**: Vérifier `gamification_handler.go`
- 🎨 **Frontend Issues**: Vérifier `UserLevelCard.tsx`
- 📊 **Data Issues**: Vérifier `gamification_service.go`
- 📚 **Docs**: Voir `GAMIFICATION_IMPLEMENTATION.md`
- ✅ **Checklist**: Voir `VALIDATION_CHECKLIST.md`

---

**Version**: 1.0.0  
**Dernière Mise à Jour**: 01 Décembre 2025  
**Statut**: Production Ready ✅
