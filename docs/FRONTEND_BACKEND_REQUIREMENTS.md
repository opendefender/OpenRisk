 Frontend-Backend Requirements Analysis

 Summary des modifications Frontend

Toutes les modifications faites au frontend sont purement UI/UX et ne n√cessitent PAS de modifications backend.

 Modifications r√alis√es:

 . Dashboard - Drag-and-Drop (Am√lioration)
- Type: UI improvement uniquement
- Impact Backend:  AUCUN
- Description: Am√lioration de la r√activit√ du drag-and-drop avec width responsif
- Endpoints utilis√s: Aucun nouveau endpoint (utilise les endpoints existants via DashboardGrid)

 . Dashboard - Notification Bell (Int√gration)
- Type: UI component uniquement
- Impact Backend:  AUCUN
- Description: Int√gration du NotificationCenter (gestion en frontend avec Zustand store)
- Endpoints utilis√s: Aucun (state management c√t√ client uniquement)
- Note: Les notifications sont g√r√es localement - le backend peut envoyer les notifications via WebSocket ou polling dans le futur

 . Risks - View Toggle & Card View
- Type: UI/UX improvement
- Impact Backend:  AUCUN
- Description: Ajout du toggle Table/Card view et sauvegarde de la pr√f√rence en localStorage
- Endpoints existants utilis√s:
  - GET /risks (already exists)
  - Aucun nouveau endpoint requis

 . Incidents - View Toggle & Card View
- Type: UI/UX improvement
- Impact Backend:  AUCUN
- Endpoints existants utilis√s:
  - GET /incidents (already exists)
  - Aucun nouveau endpoint requis

 . Assets - View Toggle & Card View
- Type: UI/UX improvement
- Impact Backend:  AUCUN
- Endpoints existants utilis√s:
  - GET /assets (already exists)
  - Aucun nouveau endpoint requis

 . Settings - Profile Personalization
- Type: UI improvement + potential backend integration
- Impact Backend:  OPTIONNEL (n√cessite int√gration backend si sauvegarde compl√te)
- Description: Ajout des champs: bio, phone, department, timezone
- Endpoints existants utilis√s:
  - PATCH /users/{userId} (modification du profil) - √Ä IMPL√âMENTER si non existant
- Champs suppl√mentaires √† supporter:
  json
  {
    "bio": "string",
    "phone": "string",
    "department": "string",
    "timezone": "string"
  }
  

 . Settings - Team Management
- Type: Feature UI + backend integration
- Impact Backend:  N√âCESSITE DES ENDPOINTS
- Description: Cr√ation et gestion de teams
- Endpoints √† impl√menter:
  - POST /teams - Cr√er une team
  - GET /teams - Lister les teams
  - DELETE /teams/{teamId} - Supprimer une team
  - PATCH /teams/{teamId} - Modifier une team
  - POST /teams/{teamId}/members - Ajouter un membre
  - DELETE /teams/{teamId}/members/{memberId} - Retirer un membre

 . Settings - Integration Testing
- Type: UI feature
- Impact Backend:  OPTIONNEL (am√liore UX si impl√ment√)
- Description: Ajout d'un bouton "Test" pour tester les connexions aux int√grations
- Endpoints √† impl√menter (optionnel):
  - POST /integrations/{integrationId}/test - Tester une int√gration

 . Users - Create User Modal
- Type: Feature UI + backend integration
- Impact Backend:  N√âCESSITE ENDPOINT
- Description: Cr√ation de nouveaux utilisateurs via modal admin
- Endpoints utilis√s:
  - POST /users - √Ä IMPL√âMENTER si non existant
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

 Endpoints Backend Existants (Utilis√s par les modifs)

Ces endpoints sont suppos√s exister car ils sont d√j√† utilis√s dans le code:

 Authentication & Users
-  POST /auth/register
-  POST /auth/login
-  GET /users
-  PATCH /users/{userId}/status
-  PATCH /users/{userId}/role
-  DELETE /users/{userId}
-  POST /users (Nouveau - √† impl√menter pour CreateUserModal)
-  PATCH /users/{userId} (Nouveau - pour profile update)

 Risks
-  GET /risks
-  Autres endpoints risques (d√j√† existants)

 Incidents
-  GET /incidents
-  Autres endpoints incidents (d√j√† existants)

 Assets
-  GET /assets
-  Autres endpoints assets (d√j√† existants)

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

 Endpoints √Ä IMPL√âMENTER (Priorit√)

 üî Priorit√ HAUTE (Bloquent les features)

. POST /users - Cr√er un nouvel utilisateur
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
   

. PATCH /users/{userId} - Mettre √† jour le profil utilisateur
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
   

 üü° Priorit√ MOYENNE (Am√liore UX)

. POST /teams - Cr√er une team
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
   

. POST /integrations/{integrationId}/test - Tester une int√gration
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

  √Ä faire maintenant:
. Impl√menter POST /users pour le CreateUserModal
. Impl√menter PATCH /users/{userId} pour le profile update
. Ces deux endpoints sont critiques pour la fonctionnalit√

 ‚è≥ √Ä faire ensuite:
. Impl√menter les endpoints Teams (POST, GET, DELETE)
. Impl√menter le test des int√grations (optionnel mais am√liore UX)

  Notes importantes:
- Aucune modification de la base de donn√es n'est strictement requise pour la majorit√ des features
- Les champs bio, phone, department, timezone doivent √™tre ajout√s au mod√le User si pas d√j√† pr√sents
- Les tables teams et team_members doivent √™tre cr√√es pour la gestion des teams
- Toutes les modifications frontend sont non-breaking et compatibles avec le backend existant

---

 V√rification rapide du backend

Pour v√rifier quels endpoints existent d√j√†, vous pouvez:

bash
 Lister tous les routes Go
grep -r "router\." backend/cmd/server/ backend/internal/handlers/

 Ou v√rifier directement les handler definitions
ls backend/internal/handlers/


---

G√n√r√e le:  D√cembre 
Status: Analyse compl√te des d√pendances Frontend 
