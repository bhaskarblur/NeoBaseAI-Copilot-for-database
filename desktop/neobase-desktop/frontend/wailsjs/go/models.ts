export namespace main {
	
	export class Connection {
	    type: string;
	    host: string;
	    port: number;
	    username: string;
	    password: string;
	    database: string;
	    ssl: boolean;
	    ssl_mode?: string;
	    connection_name: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.database = source["database"];
	        this.ssl = source["ssl"];
	        this.ssl_mode = source["ssl_mode"];
	        this.connection_name = source["connection_name"];
	    }
	}

}

