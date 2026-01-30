 Frontend-Backend Requirements Analysis

 Summary des modifications Frontend

Toutes les modifications faites au frontend sont purement UI/UX et ne ncessitent PAS de modifications backend.

 Modifications ralises:

 . Dashboard - Drag-and-Drop (Amlioration)
- Type: UI improvement uniquement
- Impact Backend:  AUCUN
- Description: Amlioration de la ractivit du drag-and-drop avec width responsif
- Endpoints utiliss: Aucun nouveau endpoint (utilise les endpoints existants via DashboardGrid)

 . Dashboard - Notification Bell (Intgration)
- Type: UI component uniquement
- Impact Backend:  AUCUN
- Description: Intgration du NotificationCenter (gestion en frontend avec Zustand store)
- Endpoints utiliss: Aucun (state management ct client uniquement)
- Note: Les notifications sont gres localement - le backend peut envoyer les notifications via WebSocket ou polling dans le futur

 . Risks - View Toggle & Card View
- Type: UI/UX improvement
- Impact Backend:  AUCUN
- Description: Ajout du toggle Table/Card view et sauvegarde de la prfrence en localStorage
- Endpoints existants utiliss:
  - GET /risks (already exists)
  - Aucun nouveau endpoint requis

 . Incidents - View Toggle & Card View
- Type: UI/UX improvement
- Impact Backend:  AUCUN
- Endpoints existants utiliss:
  - GET /incidents (already exists)
  - Aucun nouveau endpoint requis

 . Assets - View Toggle & Card View
- Type: UI/UX improvement
- Impact Backend:  AUCUN
- Endpoints existants utiliss:
  - GET /assets (already exists)
  - Aucun nouveau endpoint requis

 . Settings - Profile Personalization
- Type: UI improvement + potential backend integration
- Impact Backend:  OPTIONNEL (ncessite intgration backend si sauvegarde complte)
- Description: Ajout des champs: bio, phone, department, timezone
- Endpoints existants utiliss:
  - PATCH /users/{userId} (modification du profil) - À IMPLÉMENTER si non existant
- Champs supplmentaires à supporter:
  json
  {
    "bio": "string",
    "phone": "string",
    "department": "string",
    "timezone": "string"
  }
  

 . Settings - Team Management
- Type: Feature UI + backend integration
- Impact Backend:  NÉCESSITE DES ENDPOINTS
- Description: Cration et gestion de teams
- Endpoints à implmenter:
  - POST /teams - Crer une team
  - GET /teams - Lister les teams
  - DELETE /teams/{teamId} - Supprimer une team
  - PATCH /teams/{teamId} - Modifier une team
  - POST /teams/{teamId}/members - Ajouter un membre
  - DELETE /teams/{teamId}/members/{memberId} - Retirer un membre

 . Settings - Integration Testing
- Type: UI feature
- Impact Backend:  OPTIONNEL (amliore UX si implment)
- Description: Ajout d'un bouton "Test" pour tester les connexions aux intgrations
- Endpoints à implmenter (optionnel):
  - POST /integrations/{integrationId}/test - Tester une intgration

 . Users - Create User Modal
- Type: Feature UI + backend integration
- Impact Backend:  NÉCESSITE ENDPOINT
- Description: Cration de nouveaux utilisateurs via modal admin
- Endpoints utiliss:
  - POST /users - À IMPLÉMENTER si non existant
  - Champs requis:
    json
    {
      "email": "string",
      "full_name": "string",
      "username": "string",
      "password": "string",
      "role": "viewer|analyst|admin",
      "group": "string (optionnel)"
    }
    

---

 Endpoints Backend Existants (Utiliss par les modifs)

Ces endpoints sont supposs exister car ils sont djà utiliss dans le code:

 Authentication & Users
-  POST /auth/register
-  POST /auth/login
-  GET /users
-  PATCH /users/{userId}/status
-  PATCH /users/{userId}/role
-  DELETE /users/{userId}
-  POST /users (Nouveau - à implmenter pour CreateUserModal)
-  PATCH /users/{userId} (Nouveau - pour profile update)

 Risks
-  GET /risks
-  Autres endpoints risques (djà existants)

 Incidents
-  GET /incidents
-  Autres endpoints incidents (djà existants)

 Assets
-  GET /assets
-  Autres endpoints assets (djà existants)

 Stats & Dashboard
-  GET /stats/risk-matrix
-  GET /stats/trends
-  GET /analytics/dashboard

 Tokens
-  GET /tokens
-  POST /tokens
-  POST /tokens/{tokenId}/revoke
-  DELETE /tokens/{tokenId}
-  POST /tokens/{tokenId}/rotate

---

 Endpoints À IMPLÉMENTER (Priorit)

  Priorit HAUTE (Bloquent les features)

. POST /users - Crer un nouvel utilisateur
   bash
   POST /api/v/users
   Content-Type: application/json
   
   {
     "email": "user@example.com",
     "full_name": "John Doe",
     "username": "johndoe",
     "password": "securepassword",
     "role": "analyst",
     "group": "Security Team"
   }
   
   Response:
   {
     "id": "uuid",
     "email": "user@example.com",
     "username": "johndoe",
     "full_name": "John Doe",
     "role": "analyst",
     "is_active": true,
     "created_at": "--T..."
   }
   

. PATCH /users/{userId} - Mettre à jour le profil utilisateur
   bash
   PATCH /api/v/users/{userId}
   Content-Type: application/json
   
   {
     "full_name": "John Doe",
     "bio": "Security expert...",
     "phone": "+--",
     "department": "Security",
     "timezone": "America/New_York"
   }
   
   Response: { user_object }
   

  Priorit MOYENNE (Amliore UX)

. POST /teams - Crer une team
   bash
   POST /api/v/teams
   
   {
     "name": "Security Team",
     "description": "Main security operations team"
   }
   
   Response:
   {
     "id": "uuid",
     "name": "Security Team",
     "description": "Main security operations team",
     "members": ,
     "created_at": "--T..."
   }
   

. GET /teams - Lister les teams
   bash
   GET /api/v/teams
   
   Response:
   {
     "data": [
       { "id": "...", "name": "...", "members": , ... }
     ]
   }
   

. DELETE /teams/{teamId} - Supprimer une team
   bash
   DELETE /api/v/teams/{teamId}
   

. POST /integrations/{integrationId}/test - Tester une intgration
   bash
   POST /api/v/integrations/{integrationId}/test
   
   {
     "api_url": "https://...",
     "api_key": "..."
   }
   
   Response:
   {
     "success": true,
     "message": "Connection successful"
   }
   

---

 Recommandations

  À faire maintenant:
. Implmenter POST /users pour le CreateUserModal
. Implmenter PATCH /users/{userId} pour le profile update
. Ces deux endpoints sont critiques pour la fonctionnalit

 ⏳ À faire ensuite:
. Implmenter les endpoints Teams (POST, GET, DELETE)
. Implmenter le test des intgrations (optionnel mais amliore UX)

  Notes importantes:
- Aucune modification de la base de donnes n'est strictement requise pour la majorit des features
- Les champs bio, phone, department, timezone doivent être ajouts au modle User si pas djà prsents
- Les tables teams et team_members doivent être cres pour la gestion des teams
- Toutes les modifications frontend sont non-breaking et compatibles avec le backend existant

---

 Vrification rapide du backend

Pour vrifier quels endpoints existent djà, vous pouvez:

bash
 Lister tous les routes Go
grep -r "router\." backend/cmd/server/ backend/internal/handlers/

 Ou vrifier directement les handler definitions
ls backend/internal/handlers/


---

Gnre le:  Dcembre 
Status: Analyse complte des dpendances Frontend 
