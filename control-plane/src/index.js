// Copyright 2021 Google LLC
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
import { query, orderBy, limit, getFirestore, collectionGroup } from "firebase/firestore";
import "./button-clicks";
import { makeTable } from "./jobs-functions"
import "./styles/index.css";
import VALUES from "./values";

// Initialize Firebase (Firestore and Analytics)
fetch('/__/firebase/init.json').then(async response => {
    const app = initializeApp(await response.json());
    const db = getFirestore(app);
    VALUES.db = db;

    let url = new URL(window.location.href)
    let jobSuccess = url.searchParams.get('job')
    if (jobSuccess == "success" || jobSuccess == "editsuccess") {
        // show the toast after 3 seconds so that the material lite code css and js can load
        setTimeout(function() {
            let snackbarContainer = document.querySelector('#snackbar-container');
            snackbarContainer.MaterialSnackbar.showSnackbar({
                message: jobSuccess == "success" ? 'Job successfully added!' : 'Job successfully updated!'
            });
            // remove the job=success so that when you refresh it doesnt show the messgae again
            window.history.replaceState(null, null, window.location.pathname)
        }, 3000);
    } 

    // make the table with the first 10
    makeTable(db, query(collectionGroup(VALUES.db, VALUES.collection), orderBy('created', 'desc'), limit(10)), true)
});