import { initializeApp } from "firebase/app";
import { query, orderBy, limit, getFirestore, collectionGroup } from "firebase/firestore";
import "./button-clicks";
import { makeTable } from "./jobs-functions"
import "./styles/index.css";
import VALUES from "./values";

// Initialize Firebase (Firestore and Analytics)
const app = initializeApp(VALUES.firebaseConfig);
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