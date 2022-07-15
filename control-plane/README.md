
# Control plane

The control plane is used for controlling and monitoring the jobs.

## Get Started
1. Clone the repo
2. Run [npm](https://docs.npmjs.com/downloading-and-installing-node-js-and-npm) install
3. Use "npm start" to launch a dev server.
## Deploy to Firebase
1. [Create Firebase project](https://cloud.google.com/firestore/docs/client/get-firebase) with Hosting and Firestore
2. Copy firebaseConfig to line 9 in src/index.js
3. Install [Firebase CLI](https://firebase.google.com/docs/cli#install_the_firebase_cli)
4. Log into Firebase CLI using "firebase login"
5. Run "firebase use --add" to add an alias. When prompted, select your **Project ID**, then give your Firebase project an alias. For more info, visit this [link](https://firebase.google.com/docs/cli#add_alias).
6. Finally, run "npm run deploy" to deploy to Firebase Hosting. When you run the command, Firebase CLI should give a link to your web app.
7. If you want to run the app on your local machine, you can use "firebase emulators:start --only hosting"