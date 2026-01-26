export namespace application {
	
	export class IntervalPresetDTO {
	    label: string;
	    ms: number;
	
	    static createFrom(source: any = {}) {
	        return new IntervalPresetDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.ms = source["ms"];
	    }
	}
	export class ScriptDTO {
	    id: string;
	    name: string;
	    code: string;
	    intervalMs: number;
	    isRunning: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ScriptDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.code = source["code"];
	        this.intervalMs = source["intervalMs"];
	        this.isRunning = source["isRunning"];
	    }
	}
	export class ServerConfigDTO {
	    type: number;
	    typeName: string;
	    slaveId: number;
	    tcpAddress: string;
	    tcpPort: number;
	    serialPort: string;
	    baudRate: number;
	    dataBits: number;
	    stopBits: number;
	    parity: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.typeName = source["typeName"];
	        this.slaveId = source["slaveId"];
	        this.tcpAddress = source["tcpAddress"];
	        this.tcpPort = source["tcpPort"];
	        this.serialPort = source["serialPort"];
	        this.baudRate = source["baudRate"];
	        this.dataBits = source["dataBits"];
	        this.stopBits = source["stopBits"];
	        this.parity = source["parity"];
	    }
	}

}

