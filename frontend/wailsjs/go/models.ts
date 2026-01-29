export namespace application {
	
	export class CapabilitiesDTO {
	    supportsUnitId: boolean;
	    unitIdMin?: number;
	    unitIdMax?: number;
	
	    static createFrom(source: any = {}) {
	        return new CapabilitiesDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.supportsUnitId = source["supportsUnitId"];
	        this.unitIdMin = source["unitIdMin"];
	        this.unitIdMax = source["unitIdMax"];
	    }
	}
	export class ConditionDTO {
	    field: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new ConditionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.field = source["field"];
	        this.value = source["value"];
	    }
	}
	export class ConfigVariantDTO {
	    id: string;
	    displayName: string;
	
	    static createFrom(source: any = {}) {
	        return new ConfigVariantDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	    }
	}
	export class OptionDTO {
	    value: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new OptionDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.value = source["value"];
	        this.label = source["label"];
	    }
	}
	export class FieldDTO {
	    name: string;
	    label: string;
	    type: string;
	    required: boolean;
	    default: any;
	    options?: OptionDTO[];
	    min?: number;
	    max?: number;
	    showWhen?: ConditionDTO;
	
	    static createFrom(source: any = {}) {
	        return new FieldDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.label = source["label"];
	        this.type = source["type"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.options = this.convertValues(source["options"], OptionDTO);
	        this.min = source["min"];
	        this.max = source["max"];
	        this.showWhen = this.convertValues(source["showWhen"], ConditionDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
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
	export class MemoryAreaDTO {
	    id: string;
	    displayName: string;
	    isBit: boolean;
	    size: number;
	    readOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MemoryAreaDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.isBit = source["isBit"];
	        this.size = source["size"];
	        this.readOnly = source["readOnly"];
	    }
	}
	
	export class ProtocolConfigDTO {
	    protocolType: string;
	    variant: string;
	    settings: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ProtocolConfigDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocolType = source["protocolType"];
	        this.variant = source["variant"];
	        this.settings = source["settings"];
	    }
	}
	export class ProtocolInfoDTO {
	    type: string;
	    displayName: string;
	    variants: ConfigVariantDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ProtocolInfoDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.displayName = source["displayName"];
	        this.variants = this.convertValues(source["variants"], ConfigVariantDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class VariantDTO {
	    id: string;
	    displayName: string;
	    fields: FieldDTO[];
	
	    static createFrom(source: any = {}) {
	        return new VariantDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.fields = this.convertValues(source["fields"], FieldDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProtocolSchemaDTO {
	    protocolType: string;
	    displayName: string;
	    variants: VariantDTO[];
	    capabilities: CapabilitiesDTO;
	
	    static createFrom(source: any = {}) {
	        return new ProtocolSchemaDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocolType = source["protocolType"];
	        this.displayName = source["displayName"];
	        this.variants = this.convertValues(source["variants"], VariantDTO);
	        this.capabilities = this.convertValues(source["capabilities"], CapabilitiesDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
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
	export class UnitIDSettingsDTO {
	    min: number;
	    max: number;
	    disabledIds: number[];
	
	    static createFrom(source: any = {}) {
	        return new UnitIDSettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.min = source["min"];
	        this.max = source["max"];
	        this.disabledIds = source["disabledIds"];
	    }
	}

}

