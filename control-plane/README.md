
# Job Control plane

The job control plane is used for controlling and monitoring aggregation jobs.

## Getting set up
* Install [nodejs](https://nodejs.org/en/) + [yarn](https://yarnpkg.com/)
* Clone the repo
  * `git clone https://github.com/google/privacy-sandbox-aggregation-service`
* Navigate to control-plane folder
* Run `yarn install`
* [Create Firebase project](https://cloud.google.com/firestore/docs/client/get-firebase) with Hosting and Firestore
* Install [Firebase CLI](https://firebase.google.com/docs/cli#install_the_firebase_cli)
* Log into Firebase CLI using "firebase login"
* Run `firebase use --add` to add an alias. When prompted, select your **Project ID**, then give your Firebase project an alias. For more info, visit this [link](https://firebase.google.com/docs/cli#add_alias).
* Run `yarn start` or `yarn deploy`

## Commands
* `yarn start` - starts firebase emulator
* `yarn start-webpack` - starts webpack development server
* `yarn test` - runs jest tests on functions
* `yarn build` - builds production code to dist folder
* `yarn build-dev` - builds dev code to dist folder
* `yarn deploy` - deploys latest code to firebase hosting
  * Requires Firebase to be setup in project
* `yarn deploy-dev` - deploys dev code to firebase hosting
  * Requires Firebase to be setup in project

# How it works

## Main Architecture
It is a simple javascript web app that is integrated with [react](https://reactjs.org/) to utilize it's components feature. The backend is built with [firestore](https://firebase.google.com/docs/firestore) and [firebase authentication](https://firebase.google.com/docs/auth). The two aggregation servers will update the firestore and the jobs control plane (JCP) will then pull the jobs from firestore.

## Sorting
The sorting architecture is not yet finished; however, it is important to explain the current state of the code. Currently, when a user sorts by status, the JCP will make a collection group query on the levels collection looking for all documents that fit that status. If the status is failed, the JCP will classify the job as failed and show that job in the table. However, if the status is not failed, the JCP has to read all documents in the job to ensure that the document is truly that status. Future implementation will include Cloud functions.

Created and Updated sorts are much simpler and only require a single query to the top level document which will run on the firestore servers.

## Search
A user can search for jobs by Job ID. To search for multiple jobs, separate with semicolons.

## Table
The table UI is based on [Material Design Lite (MDL)](https://getmdl.io/). The top level of the table contains information about the job. When you click on the job, it will display information about the sub-jobs. 