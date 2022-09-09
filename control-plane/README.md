
# Job Control Plane

The job control plane (JCP) is used for controlling and monitoring aggregation jobs.

## Prerequisites
* Install [nodejs](https://nodejs.org/en/) + [yarn](https://yarnpkg.com/)
* Clone the repo
  * `git clone https://github.com/google/privacy-sandbox-aggregation-service`
* Navigate to control-plane folder
* Run `yarn install`
* [Create Firebase project](https://cloud.google.com/firestore/docs/client/get-firebase) with Hosting and Firestore
* Install [Firebase CLI](https://firebase.google.com/docs/cli#install_the_firebase_cli)
* Log into Firebase CLI using "firebase login"
* Copy .firebaserc.sample to .firebaserc and update the GCP Project ID
* Run `firebase use --add` to add an alias. When prompted, select your **Project ID**, then give your Firebase project an alias. For more info, visit this [link](https://firebase.google.com/docs/cli#add_alias).
* Run `yarn start` or `yarn deploy`

## Set up admin account
* After getting set up and deploying the JCP, go to the sign up page and create an account with the email "admin@chromium.org" and a password of your choice.
* This should create an admin account. Refresh the page if you are not directed to the jobs table
* To add or remove users, navigate to your user icon at the top right of the screen and a dropdown should appear. Click the users tab.

## Allowing Google Auth login
* Navigate to the **Auth** tab, in the [Firebase Console](https://console.firebase.google.com/)
* **Enable** Google sign in and click **Save**

## Allowing GitHub Auth login
* [Register your app](https://github.com/settings/applications/new) as a Developer Application on GitHub and get your app's **Client ID** and **Client Secret**.
* Make sure your Firebase **OAuth redirect URI** (e.g. my-app-12345.firebaseapp.com/__/auth/handler) is set as your Authorization callback URL in your app's settings page on your [GitHub app's config](https://github.com/settings/developers).

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

## Authentication / Authorization
Authentication is provided by [Firebase Authentication](https://firebase.google.com/docs/auth) while Authorization lies inside of [Cloud Firestore](https://firebase.google.com/docs/firestore) and [Firestore Rules](https://firebase.google.com/docs/firestore/security/get-started). We have four roles for all authenticated users:

* **Admin** - able to view/edit jobs and manage users
* **Editor** - able to view/edit jobs
* **Viewer** - able to view jobs
* **Pending** - able to sign in but not able to view/edit jobs

All authenticated users start as a **Pending** user and must be assigned a role by an **Admin** user.

## Main architecture
It is a simple javascript web app that is integrated with [react](https://reactjs.org/) to utilize it's components feature. The backend is built with [firestore](https://firebase.google.com/docs/firestore) and [firebase authentication](https://firebase.google.com/docs/auth). The two aggregation servers will update the firestore and the jobs control plane (JCP) will then pull the jobs from firestore.

## Sorting
The sorting architecture is not yet finished; however, it is important to explain the current state of the code. Currently, when a user sorts by status, the JCP will make a collection group query on the levels collection looking for all documents that fit that status. If the status is failed, the JCP will classify the job as failed and show that job in the table. However, if the status is not failed, the JCP has to read all documents in the job to ensure that the document is truly that status. Future implementation will include Cloud functions.

Created and Updated sorts are much simpler and only require a single query to the top level document which will run on the firestore servers.

## Search
A user can search for jobs by Job ID. To search for multiple jobs, separate with semicolons.

## User Table
The table UI is based on [Material Design Lite (MDL)](https://getmdl.io/). The top level of the table contains information about the job. When you click on the job, it will display information about the sub-jobs. 