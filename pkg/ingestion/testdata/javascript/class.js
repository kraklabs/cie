// ES6 Class
class UserService {
    constructor(db) {
        this.db = db;
    }

    getUser(id) {
        return this.db.query(id);
    }
}

export default UserService;
