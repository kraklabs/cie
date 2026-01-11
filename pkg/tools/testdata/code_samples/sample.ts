/**
 * Sample TypeScript code for testing parsing and indexing.
 */

/**
 * User interface
 */
interface User {
  id: string;
  name: string;
  email: string;
}

/**
 * Database interface for data operations
 */
interface Database {
  query<T>(sql: string, params: any[]): Promise<T[]>;
  insert(table: string, data: any): Promise<void>;
}

/**
 * User repository for managing user data
 */
class UserRepository {
  private database: Database;

  constructor(database: Database) {
    this.database = database;
  }

  /**
   * Get a user by ID
   */
  async getUser(userId: string): Promise<User | null> {
    if (!userId) {
      throw new Error('userId cannot be empty');
    }

    const results = await this.database.query<User>(
      'SELECT * FROM users WHERE id = ?',
      [userId]
    );

    return results.length > 0 ? results[0] : null;
  }

  /**
   * Create a new user
   */
  async createUser(user: User): Promise<void> {
    if (!user) {
      throw new Error('user cannot be null');
    }

    await this.database.insert('users', {
      id: user.id,
      name: user.name,
      email: user.email,
    });
  }

  /**
   * List all users
   */
  async listUsers(): Promise<User[]> {
    return this.database.query<User>('SELECT * FROM users', []);
  }
}

/**
 * Process a list of users and return statistics
 */
function processUsers(users: User[]): {
  total: number;
  emails: string[];
  names: string[];
} {
  return {
    total: users.length,
    emails: users.map((u) => u.email),
    names: users.map((u) => u.name),
  };
}

/**
 * Arrow function example for anonymous function detection
 */
const handleRequest = (req: Request) => {
  console.log('Handling request:', req);
};

/**
 * Export the repository
 */
export { UserRepository, User, processUsers, handleRequest };
