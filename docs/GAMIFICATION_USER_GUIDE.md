  Guide d'Utilisation - Gamification UI

 Accder à Your Gamification Profile

 Chemin Utilisateur

Navigation Sidebar
    Settings
       General Tab
            Your Gamification Profile (NEW!)


 Visuellement
. Cliquez sur  Settings dans la sidebar
. Assurez-vous que l'onglet General est slectionn
. Scrollez vers le bas, vous verrez " Your Gamification Profile"

---

 Composants Affichs

 ⃣ Level Card (Cercle Principal)


      Level Card Premium             
        
                                   
         Circle Badge              
           Level                  
                                   
        
      (Gradient dynamique)           
                                     
  Progression XP                     
   /  XP                       
   .%                   
  Vers niveau                       



Couleurs par Niveau:
- Niveau :  Green → Teal
- Niveau :  Blue → Cyan
- Niveau :  Purple → Indigo
- Niveau :  Pink → Rose
- Niveau +:  Orange → Red

 ⃣ Achievement Stats (Compteurs)


                               
  Risques Grs     Attnuations 



 ⃣ Badges Section

Badges Dbloqus ()

      
                                      
 Flag       Shield     Brain      Crown   
Initiator  Guardian   Strategist  Legend  
      

 = Dverrouill (Couleur jaune/or)
 = Verrouill (Couleur grise)

Hover sur badge → Affiche description


---

 Systme de Badges

 Les  Badges

| Badge | Nom | Description | Condition |
|-------|-----|-------------|-----------|
|  | Initiator | Crer votre premier risque | + risque cr |
|  | Guardian | Attnuer  risques | + mitigations compltes |
|  | Strategist | Grer plus de  risques | + risques grs |
|  | Legend | Atteindre  XP |  XP ou plus |

 Comment Dbloquer les Badges


INITIATOR (Dmarrage)
 Action: Crer votre er risque
    Dashboard > "+ New Risk"
    Remplir titre + description
    Slectionner assets (optionnel)
    Valider

GUARDIAN (Protection)
 Action: Complter  mitigations
    Ajouter mitigation à risque
    Dtails du risque > "+ Add Mitigation"
    Cocher "DONE" quand termine
   ×  fois = Badge!

STRATEGIST (Stratgie)
 Action: Grer + risques
    Crer progressivement
    Dashboard affiche votre compte

LEGEND (Matrise)
 Action: Accumuler  XP
    + XP par risque cr
    + XP par mitigation complte
    (~ risques +  mitigations)


---

 XP & Systme de Progression

 Formule de Calcul


XP = (Nombre Risques × ) + (Mitigations Compltes × )

Exemple:
-  risques crs      =  XP
-  mitigations faites =  XP
- TOTAL               =  XP → Niveau 


 Progression de Niveau


Niveau : - XP        (Base)
Niveau : - XP     (Intermdiaire)
Niveau : - XP     (Avanc)
Niveau : - XP    (Expert)
Niveau : + XP       (Matre)

Formule: Level = √(XP/) + 


 Barre de Progression

La barre affiche votre progression vers le niveau suivant:


Exemple: Niveau , / XP

[] .%


---

 Interactions Utilisateur

 Animations

. Chargement Initial
   - Skeleton loader anim
   - Dure: ~ seconde

. Barre XP
   - Anime au montage ( → X%)
   - Dure: . secondes
   - Easing: easeOut

. Badges
   - Apparaissent en cascade (dcal)
   - Chaque badge: dlai + .s
   - Glow effect si dverrouill

. Level Circle
   - Pop animation (scale)
   - Spring physics

 Hover Effects


Hover sur un Badge:
  • Border s'illumine (couleur niveau)
  • Tooltip apparat (description)
  • Lgre animation scale

Hover sur compteur stats:
  • Background s'claircie
  • Cursor devient pointer


 États Affichs


 SUCCESS (Chargement ok)
   → Affiche tous les lments

⏳ LOADING
   → Skeleton placeholder
   → Spinner (implicite via animate-pulse)

 ERROR
   → Icon AlertCircle rouge
   → Message d'erreur lisible
   → Bouton retry (manual F)


---

 Exemples de Scnarios

 Scnario : Utilisateur Nouveau


. Premier login
. Va à Settings > General
. Voit: "Vous avez  risques,  mitigations"
. Level ,  XP, % progression
.  badges dverrouills
. → Invite à crer son er risque


Action: Retour Dashboard > "+ New Risk"

---

 Scnario : Utilisateur Actif


.  risques crs,  mitigations compltes
. XP = (×) + (×) =  XP
. Level = √(/) +  ≈ Level 
. Progression vers Level : / = %
. Badges:
    Initiator (+ risque)
    Guardian ( mitigations)
    Strategist ( risques)
    Legend (besoin  XP)


Prochaine tape:  XP manquants pour Legend

---

 Scnario : User Complte une Mitigation


AVANT:
  risques,  mitigations, Level ,  XP

ACTION: Complter  mitigation

APRÈS (aprs refresh):
  risques,  mitigations, Level ,  XP
    Guardian badge dverrouille! 
    Toast notification: "Guardian Badge Unlocked!"


---

 Refresh & Mise à Jour

 Auto-Refresh
-  Pas d'auto-refresh actuellement
-  Refresh manuel: F ou Reload Page

 Mise à Jour aprs Action
. Crer risque → Dashboard retour
. Statut ne s'update pas automatiquement
. Solution: Aller Settings > General (fetch effectu)
. Ou: F pour refresh complet

 Prochainement (Backlog)
-  WebSocket events pour live update
-  Toast "XP Earned +" quand risque cr
-  Real-time stats refresh

---

 Troubleshooting

 "Impossible de charger les statistiques"

Causes Possibles:
. JWT Token expir
   → Solution: Logout > Reconnexion
. Backend non accessible
   → Solution: Vrifier docker-compose up
. Mauvais CORS
   → Solution: Vrifier allowOrigins dans main.go

Vrifier:
bash
 Terminal : Backend
docker-compose up

 Terminal : Vrifier API
curl -H "Authorization: Bearer YOUR_JWT" \
  http://localhost:/api/v/gamification/me


---

 Badges ne s'affichent pas

Cause: Icons non mappes (backend retourne icon name non gr)

Solution: Ajouter le mapping dans getBadgeIcon():
typescript
const icons: Record<string, React.ReactNode> = {
  Flag: <Target className="w- h-" />,
  // Ajouter ici si besoin:
  NewIcon: <NewIconComponent className="w- h-" />,
};


---

 XP ne s'update pas aprs cration risque

Cause: Pas d'auto-refresh

Solution Immdiate:
- Appuyez sur F
- Ou: Allez Settings > General (trigger fetch)

Solution Future: Implmentation WebSocket

---

 Rgles et Contraintes

 Rgles de Gamification

. XP
   - S'accumule, ne diminue jamais
   - + XP par risque cr
   - + XP par mitigation complte
   - Seul l'utilisateur voit ses stats

. Level
   - Bas sur XP total
   - Formule quadratique
   - Max visible: + (peut dpasser)

. Badges
   - Une fois dverrouills, ne peuvent pas être perdus
   - Conditions permanentes

. Privacy
   - Chaque user ne voit que ses stats
   - Pas de leaderboard publique (futur)

---

 Intgration avec Workflow

 Flux Typique de Travail


. LOGIN
    Directed to Dashboard

. CREATE RISK (Dashboard)
    "+ XP"
    Stats mises à jour (aprs refresh)

. ADD MITIGATION (Risk Details)
    Statut = "DONE"
    "+ XP"

. CHECK PROGRESS (Settings > General)
    Voir progression XP
    Voir badges dverrouills
    Motivation pour continuer

. REPEAT
    Plus on gre de risques
    Plus on monte de niveau
    Plus on dverrouille de badges


---

 Support & Contacte

-  Backend Issues: Vrifier gamification_handler.go
-  Frontend Issues: Vrifier UserLevelCard.tsx
-  Data Issues: Vrifier gamification_service.go
-  Docs: Voir GAMIFICATION_IMPLEMENTATION.md
-  Checklist: Voir VALIDATION_CHECKLIST.md

---

Version: ..  
Dernire Mise à Jour:  Dcembre   
Statut: Production Ready 
