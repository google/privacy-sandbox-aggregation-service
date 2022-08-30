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
import { doc, getDocs, getDoc, collection, query, startAfter, orderBy, limit, where, endBefore, collectionGroup, limitToLast } from "firebase/firestore";
import VALUES from './values.js'
import React from 'react';
import { createRoot } from 'react-dom/client';
import Jobs from './components/Jobs';


// For the jobs table
const container = document.querySelector('#render-content');
const tableRoot = createRoot(container);

// build the table using the query passed
export async function makeTable(db, thequery, first, status) {
    // show loader
    let jobs;

    // check if it is a status sort if not do a normal query
    if(status != null && status != "") {
        if (status == "failed") {
            jobs = await getFailedJobs(thequery);
        } else if(status == "search") {
            if(VALUES.searchTerm.trim() == "") {
                thequery = query(collectionGroup(VALUES.db, VALUES.collection), orderBy('created', 'desc'), limit(10));
                jobs = await getNormalJobs(db, thequery, true);
            } else {
                jobs = await getSearchJobs();
            }
        } else {
            jobs = await getOtherJobs(status, thequery);
        }
    } else {
        jobs = await getNormalJobs(db, thequery, first);
    }

    // check if the user can go to the next page
    canAdvance(jobs.length)
    // set the html of the table
    setHtml(jobs)
}

// go to next page
export function nextPage() {
    let nextQuery = null;

    // make sure that the last query is not null and user can advance
    if(VALUES.last != null && VALUES.canAdvance) {
        // get the specific query and start after the last value in the current table
        nextQuery = getQuery(startAfter(VALUES.last))

        VALUES.direction = 0;

        // if query is found then make the table
        if(nextQuery != false) {
            $('#p2').show();
            makeTable(VALUES.db, nextQuery, false, VALUES.status)
            // increment the span holding the page number
            $('.pages span').html(parseInt($('.pages span').html()) + 1);
        }
    }
}

export function prevPage() {
    let prevQuery = null;
    // make sure that prevFirst exists and that the page number is not 1
    if(VALUES.first != null && parseInt($('.pages span').html()) != 1) {
        // get the query, but this time start at the prev first
        prevQuery = getQuery(endBefore(VALUES.first), limitToLast(10))

        VALUES.direction = 1;
        
        // if the query is found, then make the table
        if(prevQuery != false) {
            $('#p2').show();
            makeTable(VALUES.db, prevQuery, false, VALUES.status)
            // increment the span holding the page number
            $('.pages span').html(parseInt($('.pages span').html()) - 1);
        }
    }
}

export function filterJobs() {
    let status = $('#status').val();
    let created = $('#created').val();
    let updated = $('#updated').val();
    if (status == "" && created == "" && updated == "") {
        // reset the table to original values
        $('.pages span').html(0);
        $('#p2').show();
        makeTable(VALUES.db, query(collection(VALUES.db, VALUES.collection), orderBy('created', 'desc'), limit(10)))
        $('.filter-side-holder').hide()
    } else {
        let filterQuery = null;
        let createdTimestamp = null;
        let updatedTimestamp = null;
        if(created != "") {
            // get the firebase timestamp so it can compare dates
            let date = new Date();
            date.setHours(date.getHours() - created)
            createdTimestamp = Timestamp.fromDate(date)
        }
        if(updated != "") {
            // get the firebase timestamp so it can compare dates
            let date = new Date();
            date.setHours(date.getHours() - updated)
            updatedTimestamp = Timestamp.fromDate(date)
        }

        VALUES.status = status;
        VALUES.createdTimestamp = createdTimestamp;
        VALUES.updatedTimestamp = updatedTimestamp;
        VALUES.direction = 0;

        // the different ways the query can be made
        if(status != "" && created == "" && updated == "") {
            filterQuery = query(collectionGroup(VALUES.db, "levels"), where("status", "==", status), orderBy('created', 'desc'), limit(10))
        } else if (status != "" && created != "") {
            filterQuery = query(collectionGroup(VALUES.db, "levels"), where("status", "==", status), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), limit(10))
        } else if (status != "" && updated != "") {
            filterQuery = query(collectionGroup(VALUES.db, "levels"), where("status", "==", status), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), limit(10))
        } else if (updated != "" && created == "") {
            filterQuery = query(collection(VALUES.db, VALUES.collection), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), limit(10))
        } else if (created != "" && updated == "") {
            filterQuery = query(collection(VALUES.db, VALUES.collection), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), limit(10))
        }

        // make the table
        if(filterQuery != null) {
            $('.pages span').html(1);
            $('#p2').show();
            makeTable(VALUES.db, filterQuery, true, status)
        }
    }
}

// get the specific query for either prevPage or nextPage and startAt or startAfter marker
function getQuery(marker, limitMarker=limit(10)) {
    // cleans the code later on
    const status = VALUES.status;
    const createdTimestamp = VALUES.createdTimestamp;
    const updatedTimestamp = VALUES.updatedTimestamp;
    const searchTerm = VALUES.searchTerm;
    const db = VALUES.db

    if(searchTerm == "") {
        if (status == "" && createdTimestamp == null && updatedTimestamp == null) {
            return query(collection(db, VALUES.collection), orderBy('created', 'desc'), marker, limitMarker)
        } else if(status != "" && createdTimestamp == null && updatedTimestamp == null) {
            return query(collectionGroup(db, "levels"), where("status", "==", status), orderBy('created', 'desc'), marker, limitMarker)
        } else if (status != "" && createdTimestamp != null) {
            return query(collectionGroup(db, "levels"), where("status", "==", status), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), marker, limitMarker)
        } else if (status != "" && updatedTimestamp != null) {
            return query(collectionGroup(db, "levels"), where("status", "==", status), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), marker, limitMarker)
        } else if (updatedTimestamp != null && createdTimestamp == null) {
            return query(collection(db, VALUES.collection), where("updated", ">=", updatedTimestamp), orderBy('updated', 'desc'), marker, limitMarker)
        } else if (createdTimestamp != null && updatedTimestamp == null) {
            return query(collection(db, VALUES.collection), where("created", ">=", createdTimestamp), orderBy('created', 'desc'), marker, limitMarker)
        }
        return false
    } else {
        return query(collection(db, VALUES.collection), where('name', ">=", searchTerm), where("name", "<", searchTerm + 'z'), orderBy('name', 'desc'), marker, limitMarker)
    }
}

function getOverrallStatus(subjobs) {
    const statusCount = { "running": 0, "scheduled": 0, "finished": 0 };
    for(let i=0; i < subjobs.length; i++) {
        statusCount[subjobs[i].status.toLowerCase()] += 1;
        if(subjobs[i].status.toLowerCase() == "failed") {
            return "failed";
        }
    }

    return decideStatus(statusCount);
}

function decideStatus(statusCount) {
    if (statusCount['running'] == 0 && statusCount['scheduled'] == 0 && statusCount['finished'] > 0) {
        // if all of them are finished, job is done
        return "finished";
    } else if (statusCount['running'] > 0) {
        // if any of the jobs are running, then the job is running
        return "running";
    } else if (statusCount['running'] == 0 && statusCount['scheduled'] > 0) {
        // if no jobs running, then the status is scheduled
        return "scheduled";
    }
    // return scheduled if not in any of these
    return "scheduled";
}

// a nice function to calculate how long ago the last update was
export function timeSince(date) {

    const seconds = Math.floor((new Date() - date) / 1000);

    let interval = seconds / 31536000;

    if (interval > 1) {
        return Math.floor(interval) + " years";
    }
    interval = seconds / 2592000;
    if (interval > 1) {
        return Math.floor(interval) + " months";
    }
    interval = seconds / 86400;
    if (interval > 1) {
        return Math.floor(interval) + " days";
    }
    interval = seconds / 3600;
    if (interval > 1) {
        return Math.floor(interval) + " hours";
    }
    interval = seconds / 60;
    if (interval > 1) {
        return Math.floor(interval) + " minutes";
    }
    return Math.floor(seconds) + " seconds";
}

async function getNormalJobs(db, thequery, first) {
     // get all the jobs returned from the query
     let values = await getJobs(db, thequery)
     // values[0] is the jobs, values[1] is the last job, values[2] is the first job
     let jobs = values[0];
     // set values for pagination
     setValues(values[1], values[2], first)

     return jobs;
}

async function getSearchJobs() {
    let jobs = []
    let docIds = VALUES.searchTerm.split(";")
    for(var i = 0; i < docIds.length;  i++) {
        const document = await getDoc(doc(VALUES.db, VALUES.collection, docIds[i]))
        if(document.exists) {
            jobs.push(await populateJob(VALUES.db, document))
        }
    }
    return jobs;
}

async function getOtherJobs(status, thequery) {
    let filled = false;
    let jobs = [];
    let levelDocs = [];
    let completedDocs = [];  // top level jobs that have already been checked
    let lastDoc;
    while(!filled) {
        let response = await getOtherDocs(jobs, levelDocs, completedDocs, lastDoc, thequery, status);
        jobs = response[0];
        levelDocs = response[1];
        completedDocs = response[2];
        filled = response[3];
        lastDoc = response[4];
    }

    return jobs;
}

async function getOtherDocs(jobs, levelDocs, completedDocs, lastDoc, thequery, status) {
    let numberOfDocs = jobs.length;
    if(lastDoc != null) {
        if(VALUES.direction != null && VALUES.direction == 1) {
            thequery = getQuery(endBefore(VALUES.first), limitToLast(10))
        } else {
            thequery = getQuery(startAfter(lastDoc));
        }
    }
    const querySnapshot = await getDocs(thequery);
    let filled = false;
    let last;
    if(querySnapshot.empty) {
        return [jobs, [], [], true]
    } else {
        filled = await Promise.all(querySnapshot.docs.map(async (document) => {
            let parentDoc = document.ref.parent.path.split('/')[1]   // get top level document
            if(completedDocs.includes(parentDoc) != true) {

                completedDocs.push(parentDoc);

                const aggregators = await getAggregators(VALUES.db, parentDoc, []);

                let subjobs = [];
                // fill in the subjobs field with the jobs logs
                subjobs = await getLevels(VALUES.db, parentDoc, aggregators[0], subjobs)
                subjobs = await getLevels(VALUES.db, parentDoc, aggregators[1], subjobs)
                
                if(getOverrallStatus(subjobs) == status) {

                    numberOfDocs += 1;

                    // when # of docs goes over amount stop it
                    if(numberOfDocs > 10) {
                        VALUES.last = document;
                        VALUES.first = levelDocs[0];
                        return true;
                    }

                    let job = {};
                    job.logs = subjobs;
                    job.status = status;
                    job.id = parentDoc;

                    const topLevelDoc = await getDoc(doc(VALUES.db, VALUES.collection, parentDoc));
                    let vals = topLevelDoc.data();
                    job.created = vals.created;
                    job.updated = vals.updated;
                    if(vals.updated == null) {
                        job.updated = job.created;
                    }

                    jobs.push(job);
                    levelDocs.push(document);
                }
                
            }
        }));
        filled = filled.at(-1)
        if(filled == undefined) {
            filled = false;
        }
        if(!filled) {
            last = querySnapshot.docs[querySnapshot.docs.length-1];
            VALUES.last = last;
            if(levelDocs.length >= 1) {
                VALUES.first = levelDocs[0];
            }
        }
    }
    
    return [jobs, levelDocs, completedDocs, filled, last];
}


async function getFailedJobs(thequery) {
    let filled = false
    let docs = [];
    let levelDocs = [];
    let lastDoc;
    while(!filled) {
        let response = await getFailedDocs(docs, levelDocs, lastDoc, thequery);
        docs = response[0];
        levelDocs = response[1];
        filled = response[2];
        lastDoc = response[3];
    }
    let jobs = [];
    for(var i = 0; i < docs.length; i++) {
        const document = await getDoc(doc(VALUES.db, VALUES.collection, docs[i]))
        jobs.push(await populateJob(VALUES.db, document));
    }

    return jobs;
}

async function getFailedDocs(failedDocs, levelDocs, lastDoc, thequery) {
    let numberOfFailedDocs = failedDocs.length;
    if(lastDoc != null){
        if(VALUES.direction != null && VALUES.direction == 1) {
            thequery = getQuery(endBefore(VALUES.first), limitToLast(10))
        } else {
            thequery = getQuery(startAfter(lastDoc));
        }
    }
    const querySnapshot = await getDocs(thequery);
    let filled = false;
    let last;
    if(querySnapshot.empty) {
        return [failedDocs, [], true]
    } else {
        filled = await Promise.all(querySnapshot.docs.map(async (document) => {
            let parentDoc = document.ref.parent.path.split('/')[1]   // get top level document
            if(failedDocs.includes(parentDoc) != true) {
                failedDocs.push(parentDoc);
                levelDocs.push(document);
                numberOfFailedDocs += 1;
                // when # of failed docs reaches ten, exit promise and save position
                if(numberOfFailedDocs >= 10) {
                    VALUES.last = document;
                    VALUES.first = levelDocs[0];
                    return true;
                }
            }
        }));
        filled = filled.at(-1)
        if(filled == undefined) {
            filled = false;
        }
        if(!filled) {
            last = querySnapshot.docs[querySnapshot.docs.length-1];
            VALUES.last = last;
            VALUES.first = levelDocs[0];
        }
    }

    return [failedDocs, levelDocs, filled, last];
}

// get the 10 jobs that are requested and the last document
async function getJobs(db, thequery) {
    let jobs = [];
    const querySnapshot = await getDocs(thequery);
    // populate the jobs array
    jobs = await populateJobs(db, jobs, querySnapshot);
    // get the last document. used for pagination
    const last = querySnapshot.docs[querySnapshot.docs.length-1];
    // get the first document. used for pagination
    const first = querySnapshot.docs[0]
    return [jobs, last, first];
}


// populate the jobs variable using an async method and in parallel
async function populateJobs(db, jobs, querySnapshot) {
    await Promise.all(querySnapshot.docs.map(async (doc) => {
        jobs.push(await populateJob(db, doc));
    }));

    return jobs;
}

async function populateJob(db, doc) {
    let job = {};
    job.id = doc.id;
    let vals = doc.data();
    job.created = vals.created;
    job.updated = vals.updated;
    if(vals.updated == null) {
        job.updated = job.created;
    }

    // fill in the aggregators field with the aggregators
    const aggregators = await getAggregators(db, doc.id, []);


    let subjobs = [];
    // fill in the subjobs field with the jobs logs 
    subjobs = await getLevels(db, doc.id, aggregators[0], subjobs)
    subjobs = await getLevels(db, doc.id, aggregators[1], subjobs)
    job.logs = subjobs;

    if(vals.overrallStatus != null) {
        // job finished or failed
        job.status = vals.overrallStatus;
    } else {
        // job scheduled or running or top level document hasn't been updated
        job.status = getOverrallStatus(subjobs)
    }

    return job;
}

// function to get the logs for a given document and aggregator
// and fill in the subjobs array and return it back
async function getLevels(db, docId, aggregator, subjobs) {
    const levelSnapshot = await getDocs(collection(db, VALUES.collection, docId, "aggregators", aggregator, "levels"));
    levelSnapshot.forEach((doc) => {
        let log = {};
        log.level = aggregator+"-"+doc.id;
        let vals = doc.data();
        log.created = vals.created;
        log.message = vals.message;
        log.result = vals.result;
        log.status = vals.status;
        log.total_levels = vals.total_levels;
        log.updated = vals.updated;
        subjobs.push(log)
    });
    return subjobs;
}

async function getAggregators(db, docId, aggregators) {
    const aggregatorSnapshot = await getDocs(collection(db, VALUES.collection, docId, "aggregators"));
    aggregatorSnapshot.forEach((doc) => {
        aggregators.push(doc.id);
    })
    return aggregators;
}

// change a firestore timestamp to a date and then format it
export function formatTime(timestamp, updated) {
    let date = timestamp.toDate();

    // if updated get the amount of time it has been since the timestamp
    if (updated) {
        return timeSince(date) + " ago";
    }
    let day = date.getDate();
    let month = date.getMonth() + 1;
    let year = date.getFullYear();
    let hour = date.getHours();
    let minute = date.getMinutes();
    let seconds = date.getSeconds();
    return month + "/" + day + "/" + year + " at " + hour + ":" + minute + ":" + seconds
}

function setValues(last, first, set) {
    VALUES.last = last;
    // if it is the very first page then prevfirst is equal to first
    if(set) {
        VALUES.prevFirst = first
    } else {
        VALUES.prevFirst = VALUES.first;
    }
    VALUES.first = first;
}

function canAdvance(length) {
    // if it is less than 10, that means that there are no more jobs left in query
    // so pagination should stop
    if(length < 10) {
        VALUES.canAdvance = false;
    } else {
        VALUES.canAdvance = true;
    }
}

function setHtml(jobs) {
    if(window.jobsTable == undefined) {
        tableRoot.render(<Jobs jobs={jobs} />);
    } else {
        window.jobsTable.updateJobs(jobs)
    }
}