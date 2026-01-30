 � Guide d'Utilisation - Gamification UI

 Acc�der à Your Gamification Profile

 Chemin Utilisateur

Navigation Sidebar
  └──  Settings
      └── General Tab
          └── � Your Gamification Profile (NEW!)


 Visuellement
. Cliquez sur  Settings dans la sidebar
. Assurez-vous que l'onglet General est s�lectionn�
. Scrollez vers le bas, vous verrez "� Your Gamification Profile"

---

 Composants Affich�s

 ⃣ Level Card (Cercle Principal)

┌─────────────────────────────────────┐
│      Level Card Premium             │
│  ┌───────────────────────────┐      │
│  │                           │      │
│  │       Circle Badge        │      │
│  │         Level            │      │
│  │                           │      │
│  └───────────────────────────┘      │
│      (Gradient dynamique)           │
│                                     │
│  Progression XP                     │
│   /  XP                       │
│  ���������� .%                   │
│  Vers niveau                       │
└─────────────────────────────────────┘


Couleurs par Niveau:
- Niveau : 🟢 Green → Teal
- Niveau : 🔵 Blue → Cyan
- Niveau : 🟣 Purple → Indigo
- Niveau : � Pink → Rose
- Niveau +: 🟠 Orange → Red

 ⃣ Achievement Stats (Compteurs)

┌──────────────────────────────────┐
│                 │              │
│  Risques G�r�s   │  Att�nuations │
└──────────────────────────────────┘


 ⃣ Badges Section

Badges D�bloqu�s ()

┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐
│  ★      │  │  ★      │  │  ◇      │  │  ◇      │
│ Flag    │  │ Shield  │  │ Brain   │  │ Crown   │
│Initiator│  │Guardian │  │Strategist│ │ Legend  │
└─────────┘  └─────────┘  └─────────┘  └─────────┘

★ = D�verrouill� (Couleur jaune/or)
◇ = Verrouill� (Couleur grise)

Hover sur badge → Affiche description


---

 Syst�me de Badges

 Les  Badges

| Badge | Nom | Description | Condition |
|-------|-----|-------------|-----------|
| � | Initiator | Cr�er votre premier risque | + risque cr�� |
|  | Guardian | Att�nuer  risques | + mitigations compl�t�es |
| 🧠 | Strategist | G�rer plus de  risques | + risques g�r�s |
| 👑 | Legend | Atteindre  XP |  XP ou plus |

 Comment D�bloquer les Badges


INITIATOR (D�marrage)
└─ Action: Cr�er votre er risque
    Dashboard > "+ New Risk"
   ✓ Remplir titre + description
   ✓ S�lectionner assets (optionnel)
   ✓ Valider

GUARDIAN (Protection)
└─ Action: Compl�ter  mitigations
    Ajouter mitigation à risque
   ✓ D�tails du risque > "+ Add Mitigation"
   ✓ Cocher "DONE" quand termin�e
   ×  fois = Badge!

STRATEGIST (Strat�gie)
└─ Action: G�rer + risques
    Cr�er progressivement
    Dashboard affiche votre compte

LEGEND (Ma�trise)
└─ Action: Accumuler  XP
   📈 + XP par risque cr��
   📈 + XP par mitigation compl�t�e
    (~ risques +  mitigations)


---

 XP & Syst�me de Progression

 Formule de Calcul


XP = (Nombre Risques × ) + (Mitigations Compl�t�es × )

Exemple:
-  risques cr��s      =  XP
-  mitigations faites =  XP
- TOTAL               =  XP → Niveau 


 Progression de Niveau


Niveau : - XP        (Base)
Niveau : - XP     (Interm�diaire)
Niveau : - XP     (Avanc�)
Niveau : - XP    (Expert)
Niveau : + XP       (Ma�tre)

Formule: Level = √(XP/) + 


 Barre de Progression

La barre affiche votre progression vers le niveau suivant:


Exemple: Niveau , / XP

[████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░░] .%


---

 Interactions Utilisateur

 Animations

. Chargement Initial
   - Skeleton loader anim�
   - Dur�e: ~ seconde

. Barre XP
   - Anim�e au montage ( → X%)
   - Dur�e: . secondes
   - Easing: easeOut

. Badges
   - Apparaissent en cascade (d�cal�)
   - Chaque badge: d�lai + .s
   - Glow effect si d�verrouill�

. Level Circle
   - Pop animation (scale)
   - Spring physics

 Hover Effects


Hover sur un Badge:
  • Border s'illumine (couleur niveau)
  • Tooltip appara�t (description)
  • L�g�re animation scale

Hover sur compteur stats:
  • Background s'�claircie
  • Cursor devient pointer


 États Affich�s


 SUCCESS (Chargement ok)
   → Affiche tous les �l�ments

⏳ LOADING
   → Skeleton placeholder
   → Spinner (implicite via animate-pulse)

 ERROR
   → Icon AlertCircle rouge
   → Message d'erreur lisible
   → Bouton retry (manual F)


---

 Exemples de Sc�narios

 Sc�nario : Utilisateur Nouveau


. Premier login
. Va à Settings > General
. Voit: "Vous avez  risques,  mitigations"
. Level ,  XP, % progression
.  badges d�verrouill�s
. → Invite à cr�er son er risque


Action: Retour Dashboard > "+ New Risk"

---

 Sc�nario : Utilisateur Actif


.  risques cr��s,  mitigations compl�t�es
. XP = (×) + (×) =  XP
. Level = √(/) +  ≈ Level 
. Progression vers Level : / = %
. Badges:
   ✓ Initiator (+ risque)
   ✓ Guardian ( mitigations)
   ✓ Strategist ( risques)
   ◇ Legend (besoin  XP)


Prochaine �tape:  XP manquants pour Legend

---

 Sc�nario : User Compl�te une Mitigation


AVANT:
└─  risques,  mitigations, Level ,  XP

ACTION: Compl�ter  mitigation

APRÈS (apr�s refresh):
└─  risques,  mitigations, Level ,  XP
   ✓ Guardian badge d�verrouille! 
   └─ Toast notification: "Guardian Badge Unlocked!"


---

 Refresh & Mise à Jour

 Auto-Refresh
-  Pas d'auto-refresh actuellement
-  Refresh manuel: F ou Reload Page

 Mise à Jour apr�s Action
. Cr�er risque → Dashboard retour
. Statut ne s'update pas automatiquement
. Solution: Aller Settings > General (fetch effectu�)
. Ou: F pour refresh complet

 Prochainement (Backlog)
- 🔄 WebSocket events pour live update
-  Toast "XP Earned +" quand risque cr��
-  Real-time stats refresh

---

 Troubleshooting

 "Impossible de charger les statistiques"

Causes Possibles:
. JWT Token expir�
   → Solution: Logout > Reconnexion
. Backend non accessible
   → Solution: V�rifier docker-compose up
. Mauvais CORS
   → Solution: V�rifier allowOrigins dans main.go

V�rifier:
bash
 Terminal : Backend
docker-compose up

 Terminal : V�rifier API
curl -H "Authorization: Bearer YOUR_JWT" \
  http://localhost:/api/v/gamification/me


---

 Badges ne s'affichent pas

Cause: Icons non mapp�es (backend retourne icon name non g�r�)

Solution: Ajouter le mapping dans getBadgeIcon():
typescript
const icons: Record<string, React.ReactNode> = {
  Flag: <Target className="w- h-" />,
  // Ajouter ici si besoin:
  NewIcon: <NewIconComponent className="w- h-" />,
};


---

 XP ne s'update pas apr�s cr�ation risque

Cause: Pas d'auto-refresh

Solution Imm�diate:
- Appuyez sur F
- Ou: Allez Settings > General (trigger fetch)

Solution Future: Impl�mentation WebSocket

---

 R�gles et Contraintes

 R�gles de Gamification

. XP
   - S'accumule, ne diminue jamais
   - + XP par risque cr��
   - + XP par mitigation compl�t�e
   - Seul l'utilisateur voit ses stats

. Level
   - Bas� sur XP total
   - Formule quadratique
   - Max visible: + (peut d�passer)

. Badges
   - Une fois d�verrouill�s, ne peuvent pas être perdus
   - Conditions permanentes

. Privacy
   - Chaque user ne voit que ses stats
   - Pas de leaderboard publique (futur)

---

 Int�gration avec Workflow

 Flux Typique de Travail


. LOGIN
   └─ Directed to Dashboard

. CREATE RISK (Dashboard)
   └─ "+ XP"
   └─ Stats mises à jour (apr�s refresh)

. ADD MITIGATION (Risk Details)
   └─ Statut = "DONE"
   └─ "+ XP"

. CHECK PROGRESS (Settings > General)
   └─ Voir progression XP
   └─ Voir badges d�verrouill�s
   └─ Motivation pour continuer

. REPEAT
   └─ Plus on g�re de risques
   └─ Plus on monte de niveau
   └─ Plus on d�verrouille de badges


---

 Support & Contacte

- 📧 Backend Issues: V�rifier gamification_handler.go
-  Frontend Issues: V�rifier UserLevelCard.tsx
-  Data Issues: V�rifier gamification_service.go
- 📚 Docs: Voir GAMIFICATION_IMPLEMENTATION.md
-  Checklist: Voir VALIDATION_CHECKLIST.md

---

Version: ..  
Derni�re Mise à Jour:  D�cembre   
Statut: Production Ready 
