// Class with methods
export class UserService {
    private db: any;

    constructor(db: any) {
        this.db = db;
    }

    getUser(userId: number): any {
        return this.db.query(userId);
    }

    createUser(name: string): any {
        return this.db.insert(name);
    }
}
