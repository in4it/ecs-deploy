import { Component, OnInit, ViewChild, Output, EventEmitter } from '@angular/core';
import { NgbModal, ModalDismissReasons } from '@ng-bootstrap/ng-bootstrap';
import { ServiceDetail, ServiceDetailService }  from './service-detail.service';

import * as moment from 'moment';

@Component({
  selector: 'app-service-detail-deploy',
  templateUrl: './deploy.component.html',
})
export class DeployChildComponent implements OnInit {

  closeResult: string;
  selectedVersion: string;
  loading: boolean = false;
  serviceName: string;
  versionMap: any;

  @ViewChild('deploy', { static: true }) deployModal : NgbModal;

  @Output() deployed: EventEmitter<any> = new EventEmitter<any>();
  @Output() deploying: EventEmitter<any> = new EventEmitter<any>();

  constructor(
    private modalService: NgbModal,
    private sds: ServiceDetailService
  ) {}

  ngOnInit(): void { 

  }
  setVersionMap(versionMap): void {
    this.versionMap = versionMap
  }

  open(serviceName, selectedVersion) {
    if(!selectedVersion) {
      return
    }
    this.serviceName = serviceName
    this.selectedVersion = selectedVersion
    this.modalService.open(this.deployModal, { windowClass: 'deploy-modal' } ).result.then((result) => {
     this.closeResult = `Closed with: ${result}`;
      if(result == "Deploy") {
        this.loading = true
        this.deploying.emit(true)
        this.sds.deploy(serviceName, selectedVersion).subscribe(data => {
          this.loading = false
          this.deploying.emit(false)
          this.deployed.emit(data)
        })
      }
    }, (reason) => {
      this.closeResult = `Dismissed ${this.getDismissReason(reason)}`;
    });
  }
  private getDismissReason(reason: any): string {
    if (reason === ModalDismissReasons.ESC) {
      return 'by pressing ESC';
    } else if (reason === ModalDismissReasons.BACKDROP_CLICK) {
      return 'by clicking on a backdrop';
    } else {
      return  `with: ${reason}`;
    }
  }
}
