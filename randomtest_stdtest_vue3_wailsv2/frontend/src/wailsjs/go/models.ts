export namespace main {
	
	export class AnalyzeInputPathRequest {
	    inputDataPath: string;
	    isFile: boolean;
	    randomnessTestStandard: string;
	    randomnessTestLevel: number;
	    pValueThreshold: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeInputPathRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inputDataPath = source["inputDataPath"];
	        this.isFile = source["isFile"];
	        this.randomnessTestStandard = source["randomnessTestStandard"];
	        this.randomnessTestLevel = source["randomnessTestLevel"];
	        this.pValueThreshold = source["pValueThreshold"];
	    }
	}
	export class AnalyzeInputPathResult {
	    isFile: boolean;
	    sampleSizeBits: number;
	    sampleCount: number;
	    minimumPassSampleCount: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalyzeInputPathResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isFile = source["isFile"];
	        this.sampleSizeBits = source["sampleSizeBits"];
	        this.sampleCount = source["sampleCount"];
	        this.minimumPassSampleCount = source["minimumPassSampleCount"];
	    }
	}
	export class PickInputPathResult {
	    path: string;
	    isFile: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PickInputPathResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.isFile = source["isFile"];
	    }
	}
	export class StartRandomnessTestRequest {
	    pValueThreshold: number;
	    qValueThreshold: number;
	    inputDataPath: string;
	    isFile: boolean;
	    randomnessTestStandard: string;
	    randomnessTestLevel: number;
	    sampleCount: number;
	
	    static createFrom(source: any = {}) {
	        return new StartRandomnessTestRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pValueThreshold = source["pValueThreshold"];
	        this.qValueThreshold = source["qValueThreshold"];
	        this.inputDataPath = source["inputDataPath"];
	        this.isFile = source["isFile"];
	        this.randomnessTestStandard = source["randomnessTestStandard"];
	        this.randomnessTestLevel = source["randomnessTestLevel"];
	        this.sampleCount = source["sampleCount"];
	    }
	}
	export class TestFormDefaults {
	    pValueThreshold: number;
	    qValueThreshold: number;
	    randomnessTestStandard: string;
	    randomnessTestLevel: string;
	    inputDataPath: string;
	    isFile: boolean;
	    sampleCount: number;
	    sampleSizeBits: number;
	    minimumPassSampleCount: number;
	
	    static createFrom(source: any = {}) {
	        return new TestFormDefaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pValueThreshold = source["pValueThreshold"];
	        this.qValueThreshold = source["qValueThreshold"];
	        this.randomnessTestStandard = source["randomnessTestStandard"];
	        this.randomnessTestLevel = source["randomnessTestLevel"];
	        this.inputDataPath = source["inputDataPath"];
	        this.isFile = source["isFile"];
	        this.sampleCount = source["sampleCount"];
	        this.sampleSizeBits = source["sampleSizeBits"];
	        this.minimumPassSampleCount = source["minimumPassSampleCount"];
	    }
	}

}

