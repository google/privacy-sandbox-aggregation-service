import { initializeApp } from "firebase/app";
import {  collection, query, orderBy, limit, getFirestore } from "firebase/firestore";
import "./button-clicks";
import { addJob, validateFields, makeTable, initUpdatePage, updateJob } from "./jobs-functions"
import "./styles/index.css";
import VALUES from "./values";
import regeneratorRuntime from "regenerator-runtime";

const firebaseConfig = {
    apiKey: "AIzaSyB3PnhacstRYpPwnfk1IiOZXjbCe96i_7A",
    authDomain: "privacyaggregate-gsoc.firebaseapp.com",
    projectId: "privacyaggregate-gsoc",
    storageBucket: "privacyaggregate-gsoc.appspot.com",
    messagingSenderId: "985023793719",
    appId: "1:985023793719:web:8b35372118aa53cd230db2"
};

// Initialize Firebase (Firestore and Analytics)
const app = initializeApp(firebaseConfig);
const db = getFirestore(app);
VALUES.db = db;

if (window.location.href.indexOf('add') != -1) {
    // ADD JOB PAGE
    $('#submit-job').click(function () {
        if (validateFields()) {
            addJob(db);
        }
    });
} else if(window.location.href.indexOf('update') != -1) {
    // UPDATE JOB PAGE
    // grab job id from url and load into html tree
    let url = new URL(window.location.href)
    let jobId = url.searchParams.get('id')
    // fill in the fields with the past code
    initUpdatePage(db, jobId)
    // update job button
    $('#update-job').click(function() {
        if (validateFields()) {
            updateJob(db, jobId);
        }
    })
} else {
    // HOME PAGE
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
    makeTable(db, query(collection(VALUES.db, "jobs"), orderBy('created', 'desc'), limit(10)), true)
}