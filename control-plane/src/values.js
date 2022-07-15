// ALL THE VALUES THAT ARE COMMONLY USED THROUGHOUT THE FILES
const collection = process.env.NODE_ENV === 'production' ? 'jobs' : 'jobs-test'

export default {
    currentLevelsOne: 0,
    currentLevelsTwo: 0,
    jobStatus: {
        "running": "directions_run",
        "failed": "error",
        "scheduled": "calendar_month",
        "finished": "done"
    },
    db: null,
    first: null,
    last: null,
    canAdvance: true,
    status: "",
    createdTimestamp: null,
    updatedTimestamp: null,
    searchTerm: "",
    collection: collection,
    direction: null
}