// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import { initializeApp } from "firebase/app";
import { query, orderBy, limit, getFirestore, collection, doc, getDoc} from "firebase/firestore";
import { getAuth, onAuthStateChanged } from "firebase/auth";
import "./button-clicks";
import { makeTable } from "./jobs-functions";
import { showAuthScreen, showWaitingScreen } from "./auth-functions";
import "./styles/index.css";
import VALUES from "./values";

// Initialize Firebase (Firestore and Analytics)
fetch('/__/firebase/init.json').then(async response => {
    const app = initializeApp(await response.json());
    const db = getFirestore(app);
    VALUES.db = db;

    // authenticate user
    const auth = getAuth();
    VALUES.auth = auth;
    onAuthStateChanged(auth, async (user) => {
        if (user) {
            VALUES.user = user;

            const docRef = doc(db, "user-management", "users", "existing-users", user.uid);
            const docSnap = await getDoc(docRef);

            if (docSnap.exists()) {
                const userData = docSnap.data();
                VALUES.userRole = userData.role;
            } else {
                showWaitingScreen();
                return;
            }

            // make the table with the first 10
            makeTable(db, query(collection(db, VALUES.collection), orderBy('created', 'desc'), limit(10)), true) 
        } else {
            // User is signed out
            showAuthScreen();
            return;
        }
    });
});